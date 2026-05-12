package rohlik

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// CategoryNode is a node in Rohlik's navigation tree.
//
// Both downward (SubcategoryIDs) and upward (ParentID) links are stored.
// ParentID is set when the node is first discovered as a child during the
// single-ID v4 BFS, which makes the parent linkage unambiguous (every child
// returned by v4(X) is a child of X). ParentID == 0 marks a top-level root.
type CategoryNode struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	ParentID       int64   `json:"parentId,omitempty"`
	SubcategoryIDs []int64 `json:"subcategoryIds,omitempty"`
}

// CategoryTree is the cached navigation hierarchy. Roots lists the
// top-level category IDs (those with ParentID == 0).
type CategoryTree struct {
	Nodes map[int64]*CategoryNode `json:"nodes"`
	Roots []int64                 `json:"roots"`
}

// SecondLevel returns the depth-1 ancestor of id — the node whose parent is
// a top-level root. Returns:
//
//   - nil if id is unknown, is a root itself, or has a broken parent chain;
//   - the node itself if id IS already at depth 1.
func (t *CategoryTree) SecondLevel(id int64) *CategoryNode {
	if t == nil {
		return nil
	}
	cur := t.Nodes[id]
	if cur == nil || cur.ParentID == 0 {
		return nil
	}
	seen := map[int64]bool{cur.ID: true}
	for {
		parent := t.Nodes[cur.ParentID]
		if parent == nil {
			return nil
		}
		if parent.ParentID == 0 {
			return cur
		}
		if seen[parent.ID] {
			return nil
		}
		seen[parent.ID] = true
		cur = parent
	}
}

// TreeProgress is called periodically during tree build. `done` is the
// running node count; `pending` is unused (kept for API compatibility).
type TreeProgress func(done, pending int)

// navTreeCacheKey is versioned: bump the suffix on any change to the on-disk
// shape or the build algorithm so stale caches auto-rebuild.
// v3..v6: incremental BFS attempts based on v5 flat + batched v4.
// v7: full top-down BFS with single-ID v4 calls per node and stored
//     ParentID. Single-ID responses make parent linkage unambiguous and
//     reliably populate each node's SubcategoryIDs.
const navTreeCacheKey = "navigation-tree-v7"

// Raw-response cache keys. These cache the *bytes* returned by the v5/v4
// nav endpoints so that a tree rebuild (e.g. after bumping navTreeCacheKey)
// reuses already-fetched HTTP responses instead of re-hitting Rohlik
// thousands of times. Each per-ID v4 response is a leaf cache entry; the
// v5 tops list is a single entry. The keys are intentionally unversioned:
// the upstream JSON shape we depend on (id, name, subcategoryIds) has been
// stable for years, and FileCache.PutRaw is a no-op once the key exists,
// so the cache is sticky.
const (
	navTopsRawCacheKey   = "nav-tops-v5-raw"
	navSubRawCacheKeyFmt = "nav-subs-v4-raw|%d"
)

func navSubRawCacheKey(id int64) string {
	return fmt.Sprintf(navSubRawCacheKeyFmt, id)
}

const (
	// navConcurrency caps how many in-flight v4 requests overlap. The
	// effective request rate is gated by navRequestInterval, so this just
	// keeps the HTTP pipeline filled while other workers wait on a token.
	navConcurrency = 4

	// navRequestInterval is the minimum gap between v4 calls across ALL
	// workers (token bucket, bucket size 1). 150 ms = ~6.7 RPS — well under
	// what Rohlik's Cloudflare layer flags. Earlier per-worker pacing
	// (8 workers × 100–200 ms each) bursted to ~40+ RPS and tripped 403s.
	navRequestInterval = 150 * time.Millisecond
)

type navTopItem struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	SubcategoryIDs []int64 `json:"subcategoryIds"`
}

type navTopResp struct {
	Items []navTopItem `json:"items"`
}

type navSubItem struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	SubcategoryIDs []int64 `json:"subcategoryIds"`
}

// LoadCategoryTree returns the cached tree if present, otherwise builds it.
func (c *Client) LoadCategoryTree(ctx context.Context, onProgress TreeProgress) (*CategoryTree, error) {
	var cached CategoryTree
	if c.cache.Get(navTreeCacheKey, &cached) && len(cached.Nodes) > 0 {
		if onProgress != nil {
			onProgress(len(cached.Nodes), 0)
		}
		return &cached, nil
	}
	return c.BuildCategoryTree(ctx, onProgress)
}

// BuildCategoryTree fetches the full nav tree top-down.
//
//   - v5 gives the top-level category IDs and names.
//   - For every discovered node, we call v4 with `?categoryIds=ID` (single
//     ID). The response gives us that node's direct children — each with
//     their own ID, Name, and SubcategoryIDs. We record `ParentID = id` for
//     each child, so the parent linkage is unambiguous, and we recurse.
//   - `navConcurrency` workers run concurrently, all sharing a global token
//     bucket (`navRequestInterval`) so the effective request rate stays
//     under Cloudflare's threshold regardless of worker count.
//   - Transient 403/429 responses are retried with exponential backoff
//     (`fetchSubcategoriesWithRetry`).
//
// v5's flat subcategoryIds and the v4 "echo of requested ID with subs"
// pattern are both handled: if the response includes the requested ID
// (with its own SubcategoryIDs listed) we treat those IDs as children
// alongside any separate child items in the response.
func (c *Client) BuildCategoryTree(ctx context.Context, onProgress TreeProgress) (*CategoryTree, error) {
	tree := &CategoryTree{Nodes: map[int64]*CategoryNode{}}

	tops, err := c.fetchTopCategories(ctx)
	if err != nil {
		return nil, err
	}
	v5FlatPerTop := map[int64][]int64{}
	for _, t := range tops.Items {
		tree.Nodes[t.ID] = &CategoryNode{ID: t.ID, Name: t.Name}
		tree.Roots = append(tree.Roots, t.ID)
		v5FlatPerTop[t.ID] = t.SubcategoryIDs
	}

	// Token bucket: one token per navRequestInterval, bucket size 1 (no
	// burst). Workers receive before each v4 call.
	limiterCtx, cancelLimiter := context.WithCancel(ctx)
	defer cancelLimiter()
	tokens := newNavRateLimiter(limiterCtx, navRequestInterval)

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		sem      = make(chan struct{}, navConcurrency)
		seen     = make(map[int64]bool, 4096)
		firstErr atomic.Pointer[error]
	)
	for _, t := range tops.Items {
		seen[t.ID] = true
	}

	var visit func(id int64)
	visit = func(id int64) {
		defer wg.Done()
		if firstErr.Load() != nil {
			return
		}

		// 1) Raw-response cache: skip sem + rate-limit token entirely on hit.
		//    This is what makes subsequent rebuilds near-instant — we only
		//    pay the network cost for IDs we've never fetched before.
		var children []navSubItem
		fromCache := false
		cacheKey := navSubRawCacheKey(id)
		if raw, ok := c.cache.GetRaw(cacheKey); ok {
			if err := json.Unmarshal(raw, &children); err == nil {
				fromCache = true
			} else {
				debugf("nav sub cache corrupt for id=%d: %v (refetching)", id, err)
				children = nil
			}
		}

		// 2) Network path (only when cache missed).
		if !fromCache {
			sem <- struct{}{}
			select {
			case <-tokens:
			case <-ctx.Done():
				<-sem
				err := ctx.Err()
				firstErr.CompareAndSwap(nil, &err)
				return
			}
			var err error
			children, err = c.fetchSubcategoriesWithRetry(ctx, []int64{id})
			<-sem

			if err != nil {
				firstErr.CompareAndSwap(nil, &err)
				return
			}

			// Persist the parsed response. Empty results are cached too — a
			// leaf node never gets re-queried on rebuild. FileCache.PutRaw is
			// no-op-on-exists, so concurrent visits for the same id are safe.
			if raw, mErr := json.Marshal(children); mErr == nil {
				if pErr := c.cache.PutRaw(cacheKey, raw); pErr != nil {
					debugf("nav sub cache put failed for id=%d: %v", id, pErr)
				}
			}
		}

		// Collect (childID, childName) pairs from the response. v4 may return
		// either the children directly, an echo of the requested ID with its
		// SubcategoryIDs populated, or both — handle all three uniformly.
		childIDs := []int64{}
		childNames := map[int64]string{}
		seenInBatch := map[int64]bool{}
		for _, ch := range children {
			if ch.ID == id {
				for _, cid := range ch.SubcategoryIDs {
					if !seenInBatch[cid] {
						seenInBatch[cid] = true
						childIDs = append(childIDs, cid)
					}
				}
				continue
			}
			if !seenInBatch[ch.ID] {
				seenInBatch[ch.ID] = true
				childIDs = append(childIDs, ch.ID)
			}
			if ch.Name != "" {
				childNames[ch.ID] = ch.Name
			}
		}

		var newIDs []int64
		var nodeCount int
		mu.Lock()
		for _, cid := range childIDs {
			if existing, ok := tree.Nodes[cid]; ok {
				if existing.Name == "" {
					existing.Name = childNames[cid]
				}
				continue
			}
			tree.Nodes[cid] = &CategoryNode{
				ID: cid, Name: childNames[cid], ParentID: id,
			}
			if !seen[cid] {
				seen[cid] = true
				newIDs = append(newIDs, cid)
			}
		}
		if n, ok := tree.Nodes[id]; ok {
			n.SubcategoryIDs = childIDs
		}
		nodeCount = len(tree.Nodes)
		mu.Unlock()

		if onProgress != nil {
			onProgress(nodeCount, 0)
		}

		for _, nid := range newIDs {
			wg.Add(1)
			go visit(nid)
		}
	}

	for _, t := range tops.Items {
		wg.Add(1)
		go visit(t.ID)
	}
	wg.Wait()

	if errp := firstErr.Load(); errp != nil {
		return nil, *errp
	}

	// Last-resort backfill from v5's flat list: any ID v5 promotes that the
	// BFS somehow didn't reach (rare; happens only if v4 returns empty for
	// some intermediate node) is attached directly to its top as a depth-1
	// orphan, so it still shows as itself rather than vanishing.
	mu.Lock()
	for topID, sids := range v5FlatPerTop {
		for _, sid := range sids {
			if _, exists := tree.Nodes[sid]; exists {
				continue
			}
			tree.Nodes[sid] = &CategoryNode{ID: sid, ParentID: topID}
		}
	}
	mu.Unlock()

	if err := c.cache.Put(navTreeCacheKey, tree); err != nil {
		return tree, fmt.Errorf("built tree but failed to cache: %w", err)
	}
	return tree, nil
}

// newNavRateLimiter returns a channel that emits one token every interval.
// Bucket size is 1 (no burst). The producer goroutine exits when ctx is
// cancelled. The first call doesn't wait — the bucket is pre-seeded.
func newNavRateLimiter(ctx context.Context, interval time.Duration) <-chan struct{} {
	tokens := make(chan struct{}, 1)
	tokens <- struct{}{} // seed
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case tokens <- struct{}{}:
				default:
				}
			}
		}
	}()
	return tokens
}

// fetchSubcategoriesWithRetry wraps fetchSubcategories with exponential
// backoff on Cloudflare/rate-limit responses (HTTP 403, 429, or the
// "Just a moment..." challenge page). Returns the original error for any
// other failure.
func (c *Client) fetchSubcategoriesWithRetry(ctx context.Context, ids []int64) ([]navSubItem, error) {
	// One attempt per backoff value, plus a final attempt with no wait after.
	backoffs := []time.Duration{2 * time.Second, 5 * time.Second, 12 * time.Second}
	attempts := len(backoffs) + 1

	var lastErr error
	for i := 0; i < attempts; i++ {
		subs, err := c.fetchSubcategories(ctx, ids)
		if err == nil {
			if i > 0 {
				debugf("fetchSubcategories ids=%v: recovered after %d retries", ids, i)
			}
			return subs, nil
		}
		lastErr = err
		if !isRateLimitErr(err) {
			return nil, err
		}
		if i == attempts-1 {
			break
		}
		wait := backoffs[i] + time.Duration(rand.IntN(1000))*time.Millisecond
		debugf("fetchSubcategories ids=%v: rate-limited (attempt %d/%d), backing off %s: %v",
			ids, i+1, attempts, wait, err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
	return nil, fmt.Errorf("rate-limited after %d attempts: %w", attempts, lastErr)
}

func isRateLimitErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "status 403") ||
		strings.Contains(s, "status 429") ||
		strings.Contains(s, "Just a moment")
}

func (c *Client) fetchTopCategories(ctx context.Context) (*navTopResp, error) {
	if raw, ok := c.cache.GetRaw(navTopsRawCacheKey); ok {
		var out navTopResp
		if err := json.Unmarshal(raw, &out); err == nil && len(out.Items) > 0 {
			debugf("nav tops: cache HIT (%d items)", len(out.Items))
			return &out, nil
		}
		debugf("nav tops: cache entry present but unusable; refetching")
	}
	body, err := c.getJSON(ctx, "https://www.rohlik.cz/api/v5/navigation/components/navigation-tabs/categories")
	if err != nil {
		return nil, err
	}
	var out navTopResp
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}
	if putErr := c.cache.PutRaw(navTopsRawCacheKey, body); putErr != nil {
		debugf("nav tops: cache put failed: %v", putErr)
	}
	return &out, nil
}

func (c *Client) fetchSubcategories(ctx context.Context, ids []int64) ([]navSubItem, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = "categoryIds=" + strconv.FormatInt(id, 10)
	}
	apiURL := "https://www.rohlik.cz/api/v4/navigation/components/navigation-tabs/subcategories?" + strings.Join(parts, "&")
	body, err := c.getJSON(ctx, apiURL)
	if err != nil {
		return nil, err
	}
	var out []navSubItem
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}
	return out, nil
}

func (c *Client) getJSON(ctx context.Context, url string) ([]byte, error) {
	req, err := c.newRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return io.ReadAll(resp.Body)
}
