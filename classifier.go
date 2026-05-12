package main

import (
	"context"
	"parec/rohlik"
	"regexp"
	"strings"
)

const unclassified = "Nezařazeno"

// reTrailingSize matches a trailing "size suffix" like " 250g", " 1,5 l",
// " 1ks", " 10 ks". Stripping it usually turns a verbose product name into
// a query that returns autocomplete hits.
var reTrailingSize = regexp.MustCompile(`(?i)\s+\d+([.,]\d+)?\s*(g|kg|ml|l|ks)\s*$`)

// ItemProgress is called once per item after classification. May be nil.
type ItemProgress func(*Item)

// ClassifyItems enriches each item in-place with MainCategory and SubCategory.
//
//   - Main = autocomplete `topParentName` (always present on a matched
//     category; Rohlik's authoritative top-level label).
//   - Sub  = `tree.SecondLevel(catID).Name` — depth-1 ancestor of the matched
//     leaf in the cached navigation tree. Falls back to the leaf name when
//     the tree is nil or doesn't know the leaf.
//
// If the verbose name produces zero autocomplete hits we retry once with
// the trailing size suffix stripped.
func ClassifyItems(
	ctx context.Context,
	client *rohlik.Client,
	tree *rohlik.CategoryTree,
	items []*Item,
	onProgress ItemProgress,
) {
	for _, item := range items {
		classifyOne(ctx, client, tree, item)
		if onProgress != nil {
			onProgress(item)
		}
	}
}

func classifyOne(ctx context.Context, client *rohlik.Client, tree *rohlik.CategoryTree, item *Item) {
	item.MainCategory = unclassified
	item.SubCategory = unclassified

	ac := lookupAutocomplete(ctx, client, item.Name)
	if ac == nil || len(ac.Categories) == 0 {
		return
	}

	cat := ac.Categories[0]

	if cat.TopParentName != "" {
		item.MainCategory = cat.TopParentName
	} else {
		item.MainCategory = cat.Name
	}

	if sub := tree.SecondLevel(cat.ID); sub != nil {
		item.SubCategory = sub.Name
	} else {
		item.SubCategory = cat.Name
	}
}

// lookupAutocomplete returns the first autocomplete response containing a
// non-empty Categories list. Tries the verbatim name first, then a retry
// with the trailing size suffix stripped. Returns nil if both attempts
// errored or yielded nothing.
func lookupAutocomplete(ctx context.Context, client *rohlik.Client, name string) *rohlik.AutocompleteResult {
	if ac, err := client.AutocompleteFull(ctx, name); err == nil && hasCategories(ac) {
		return ac
	}
	trimmed := strings.TrimSpace(reTrailingSize.ReplaceAllString(name, ""))
	if trimmed == "" || trimmed == name {
		return nil
	}
	if ac, err := client.AutocompleteFull(ctx, trimmed); err == nil && hasCategories(ac) {
		return ac
	}
	return nil
}

func hasCategories(ac *rohlik.AutocompleteResult) bool {
	return ac != nil && len(ac.Categories) > 0
}
