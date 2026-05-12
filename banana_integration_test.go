//go:build integration

// Run with:  go test -tags=integration ./...
//
// Requires:
//   - network access to https://www.rohlik.cz
//   - ./data/cookies.txt populated by a prior run (so we inherit the
//     Cloudflare clearance cookie without invoking chromedp)
//   - ./data/.cache.json present (the cached navigation-tree-v7 entry).
//     If absent, the test will rebuild it, which takes a few minutes.
//
// Verifies that resolving categories by product ID (the /api/v1/products
// path) yields the same two-level breakdown that autocomplete-by-name does
// for the canonical example "Banán 1 ks" (id 1349777).

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"parec/rohlik"
)

func TestBananaProductCategories_Integration(t *testing.T) {
	const (
		productID        int64 = 1349777 // Banán 1 ks
		wantMainCategory       = "Ovoce a zelenina"
		wantSubCategory        = "Ovoce"
	)

	// Force debug logs from the rohlik package on for this test run.
	rohlik.Debug = true

	logStep := func(msg string, args ...any) {
		t.Logf("[%s] "+msg, append([]any{time.Now().Format("15:04:05.000")}, args...)...)
	}

	if _, err := os.Stat("./data/cookies.txt"); err != nil {
		t.Skipf("./data/cookies.txt missing — run `parec` once to populate Cloudflare clearance cookies before this test: %v", err)
	}
	if st, err := os.Stat("./data/.cache.json"); err == nil {
		logStep("cache file: %s (%d bytes)", st.Name(), st.Size())
		probeTreeCache(t, "./data/.cache.json")
	} else {
		logStep("no cache file yet: %v", err)
	}

	// Cold-build the tree can legitimately take 20+ minutes under
	// Cloudflare's nav-API rate limiting. Override via PAREC_TEST_DEADLINE
	// (Go duration). The raw caches are populated incrementally, so a
	// timed-out run is not wasted — the next run resumes from disk.
	deadline := 30 * time.Minute
	if s := os.Getenv("PAREC_TEST_DEADLINE"); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			deadline = d
		}
	}
	logStep("test deadline: %s (override via PAREC_TEST_DEADLINE)", deadline)
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()

	logStep("NewClient…")
	client, err := rohlik.NewClient("./data/.cache.json")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	logStep("LoadCookies…")
	if err := client.LoadCookies(); err != nil {
		t.Fatalf("LoadCookies: %v", err)
	}

	logStep("LoadCategoryTree start (cold build can take several minutes)…")
	treeStart := time.Now()
	lastTick := time.Now()
	tree, err := client.LoadCategoryTree(ctx, func(done, _ int) {
		if done%50 == 0 || time.Since(lastTick) > 5*time.Second {
			lastTick = time.Now()
			logStep("  tree progress: %d nodes (elapsed %s)", done, time.Since(treeStart).Round(time.Second))
		}
	})
	if err != nil {
		t.Fatalf("LoadCategoryTree: %v", err)
	}
	logStep("LoadCategoryTree done: %d nodes, %d roots in %s",
		len(tree.Nodes), len(tree.Roots), time.Since(treeStart).Round(time.Millisecond))

	logStep("GetProduct(%d)…", productID)
	pStart := time.Now()
	p, err := client.GetProduct(ctx, productID)
	if err != nil {
		t.Fatalf("GetProduct(%d): %v", productID, err)
	}
	logStep("GetProduct done in %s: name=%q mainCategoryId=%d",
		time.Since(pStart).Round(time.Millisecond), p.Name, p.MainCategoryID)

	if chain := formatBananaChain(tree, p.MainCategoryID); chain != "" {
		logStep("tree chain for mainCategoryId=%d:\n%s", p.MainCategoryID, chain)
	} else {
		logStep("WARNING: mainCategoryId=%d not present in tree (%d nodes total)",
			p.MainCategoryID, len(tree.Nodes))
	}

	logStep("ProductCategories(%d)…", productID)
	pcStart := time.Now()
	top, sub, err := client.ProductCategories(ctx, tree, productID)
	if err != nil {
		t.Fatalf("ProductCategories(%d): %v", productID, err)
	}
	logStep("ProductCategories done in %s: top=%q  sub=%q",
		time.Since(pcStart).Round(time.Millisecond), top, sub)

	if top != wantMainCategory {
		t.Errorf("top = %q, want %q", top, wantMainCategory)
	}
	if sub != wantSubCategory {
		t.Errorf("sub = %q, want %q", sub, wantSubCategory)
	}
}

// probeTreeCache peeks at the on-disk cache to report whether the
// navigation-tree-v7 entry exists without invoking the rohlik package.
// Helps distinguish "tree cold-build hang" from real bugs at a glance.
func probeTreeCache(t *testing.T, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Logf("  cache read failed: %v", err)
		return
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Logf("  cache decode failed: %v", err)
		return
	}
	var navKeys []string
	for k := range m {
		if strings.HasPrefix(k, "navigation-tree") {
			navKeys = append(navKeys, k)
		}
	}
	if len(navKeys) == 0 {
		t.Logf("  cache has %d keys, NO navigation-tree-* entry → tree will be built from scratch", len(m))
		return
	}
	t.Logf("  cache has %d keys, navigation-tree entries: %v", len(m), navKeys)
}

func formatBananaChain(tree *rohlik.CategoryTree, id int64) string {
	if _, ok := tree.Nodes[id]; !ok {
		return ""
	}
	var chain []*rohlik.CategoryNode
	for cur := tree.Nodes[id]; cur != nil; cur = tree.Nodes[cur.ParentID] {
		chain = append([]*rohlik.CategoryNode{cur}, chain...)
		if cur.ParentID == 0 {
			break
		}
	}
	var b strings.Builder
	for i, n := range chain {
		fmt.Fprintf(&b, "  %sid=%d  name=%q  parentId=%d\n",
			strings.Repeat("  ", i), n.ID, n.Name, n.ParentID)
	}
	return b.String()
}
