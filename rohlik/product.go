package rohlik

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Debug toggles verbose stderr logging for product/category lookups.
// Enabled when PAREC_DEBUG is set (any non-empty value).
var Debug = os.Getenv("PAREC_DEBUG") != ""

func debugf(format string, args ...any) {
	if Debug {
		log.Printf("[rohlik] "+format, args...)
	}
}

// ErrCategoryNotFound signals that a product's category could not be
// located in the cached navigation tree.
var ErrCategoryNotFound = errors.New("category not found in tree")

// Product is a slim view of Rohlik's /api/v1/products response.
type Product struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	MainCategoryID int64  `json:"mainCategoryId"`
}

// GetProduct fetches product metadata for a single product ID via
// /api/v1/products?products=<id>. The endpoint requires no auth and returns
// an array; we take the first element. Results are cached under
// "product|<id>".
func (c *Client) GetProduct(ctx context.Context, id int64) (*Product, error) {
	if id <= 0 {
		return nil, errors.New("product id must be positive")
	}

	cacheKey := "product|" + strconv.FormatInt(id, 10)
	var cached Product
	if c.cache.Get(cacheKey, &cached) && cached.ID != 0 {
		debugf("GetProduct(%d): cache HIT (name=%q, mainCategoryId=%d)", id, cached.Name, cached.MainCategoryID)
		cp := cached
		return &cp, nil
	}
	debugf("GetProduct(%d): cache MISS, fetching", id)

	apiURL := "https://www.rohlik.cz/api/v1/products?products=" + strconv.FormatInt(id, 10)
	debugf("GET %s", apiURL)
	req, err := c.newRequest(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	start := time.Now()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		debugf("HTTP error after %s: %v", time.Since(start).Round(time.Millisecond), err)
		return nil, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()
	debugf("response: status=%d in %s", resp.StatusCode, time.Since(start).Round(time.Millisecond))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var decoded []Product
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}
	if len(decoded) == 0 {
		return nil, fmt.Errorf("no product found for id %d", id)
	}

	p := decoded[0]
	debugf("GetProduct(%d): got name=%q mainCategoryId=%d", id, p.Name, p.MainCategoryID)
	if err := c.cache.Put(cacheKey, p); err != nil {
		return &p, fmt.Errorf("fetched OK but failed to cache: %w", err)
	}
	return &p, nil
}

// ProductCategories returns the top-level (root) category name and the
// second-level (depth-1) sub-category name for a given product ID.
//
// It resolves the product's MainCategoryID via /api/v1/products and then
// walks the supplied navigation tree:
//
//   - top = the root ancestor's Name;
//   - sub = the depth-1 ancestor's Name (i.e. tree.SecondLevel(mainCategoryID)).
//
// If MainCategoryID is itself a root, sub equals top. If the tree doesn't
// know MainCategoryID, ErrCategoryNotFound is returned.
func (c *Client) ProductCategories(ctx context.Context, tree *CategoryTree, id int64) (top, sub string, err error) {
	if tree == nil {
		return "", "", errors.New("tree is required")
	}
	p, err := c.GetProduct(ctx, id)
	if err != nil {
		return "", "", err
	}
	if p.MainCategoryID == 0 {
		return "", "", fmt.Errorf("product %d has no mainCategoryId", id)
	}

	leaf, ok := tree.Nodes[p.MainCategoryID]
	if !ok {
		debugf("ProductCategories: tree has %d nodes but no entry for mainCategoryId=%d", len(tree.Nodes), p.MainCategoryID)
		return "", "", fmt.Errorf("%w: id=%d not in tree", ErrCategoryNotFound, p.MainCategoryID)
	}
	debugf("leaf: id=%d name=%q parentId=%d", leaf.ID, leaf.Name, leaf.ParentID)

	root, depthOne := walkChain(tree, leaf)
	if root == nil {
		return "", "", fmt.Errorf("%w: broken parent chain from id=%d", ErrCategoryNotFound, p.MainCategoryID)
	}
	debugf("root: id=%d name=%q  depth-1: id=%d name=%q", root.ID, root.Name, depthOne.ID, depthOne.Name)
	return root.Name, depthOne.Name, nil
}

// walkChain returns the root ancestor and the depth-1 ancestor of leaf in
// one pass up the ParentID chain. If leaf is itself a root, root == leaf
// and depthOne == leaf (sub collapses to top, which is the only sensible
// answer when the product's category is already at depth 0).
func walkChain(tree *CategoryTree, leaf *CategoryNode) (root, depthOne *CategoryNode) {
	if leaf == nil {
		return nil, nil
	}
	if leaf.ParentID == 0 {
		return leaf, leaf
	}
	seen := map[int64]bool{leaf.ID: true}
	cur := leaf
	for {
		parent := tree.Nodes[cur.ParentID]
		if parent == nil {
			return nil, nil
		}
		if parent.ParentID == 0 {
			return parent, cur
		}
		if seen[parent.ID] {
			return nil, nil
		}
		seen[parent.ID] = true
		cur = parent
	}
}
