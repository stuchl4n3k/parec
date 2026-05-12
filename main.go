package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"parec/progress"
	"parec/rohlik"
	"strings"
	"time"

	"github.com/akamensky/argparse"
)

const (
	prog        = "parec"
	version     = "1.0"
	author      = "stuchl4n3k"
	description = "parec - Parser of receipts v" + version + " by " + author
	cachePath   = "./data/.cache.json"
)

// parec pulls delivered Rohlik orders, classifies each line item via Rohlik's
// autocomplete API, and prints spend breakdowns:
//   - per month (from order totals)
//   - per main category and subcategory (as a tree)
func main() {
	parser := argparse.NewParser(prog, description)
	months := parser.Int("m", "months", &argparse.Options{
		Required: false, Default: 2, Help: "how many calendar months of orders to include, counting back from today inclusive " +
			"(e.g. --months=2 on May 11 covers April + May)",
	})

	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Fprint(os.Stderr, parser.Usage(err))
		os.Exit(1)
	}

	if *months < 1 {
		fatalf("--months must be >= 1")
	}

	if os.Getenv("ROHLIK_EMAIL") == "" || os.Getenv("ROHLIK_PASSWORD") == "" {
		fatalf("ROHLIK_EMAIL and ROHLIK_PASSWORD must be set")
	}

	ctx := context.Background()

	client, err := rohlik.NewClient(cachePath)
	if err != nil {
		fatalf("create rohlik client: %v", err)
	}
	if _, _, err := client.Auth(); err != nil {
		fatalf("auth: %v", err)
	}

	// Load the navigation tree (used to map autocomplete leaf categories to
	// their depth-1 ancestor, which is the useful "subcategory" bucket).
	// Tree-load failure is non-fatal — classification falls back to leaf
	// names if the tree is unavailable.
	treeBar := progress.New("Loading category tree", 0)
	tree, err := client.LoadCategoryTree(ctx, func(done, _ int) {
		treeBar.Set(done)
	})
	treeBar.Done()
	if err != nil {
		log.Printf("category tree unavailable (%v); subs will fall back to leaf names", err)
		tree = nil
	} else {
		log.Printf("Category tree: %d nodes across %d top-level roots.", len(tree.Nodes), len(tree.Roots))
	}

	cutoff := monthsBackCutoff(time.Now(), *months)
	log.Printf("Fetching orders since %s (%d month(s) back).", cutoff.Format("2006-01-02"), *months)

	orders, err := client.GetOrdersSince(ctx, cutoff)
	if err != nil {
		fatalf("get orders: %v", err)
	}
	log.Printf("Fetched %d orders.", len(orders))

	fmt.Println()
	fmt.Println("== Spend per month ==")
	for _, b := range AggregateOrdersByMonth(orders) {
		fmt.Println(b)
	}

	// Receipts: download + parse all (IO-bound). One bar across orders.
	receiptBar := progress.New("Downloading receipts", len(orders))
	var allItems []*Item
	for _, o := range orders {
		receiptBar.Suffix(fmt.Sprintf("#%d", o.ID))
		content, err := client.GetOrderReceipt(ctx, o.ID)
		if err != nil {
			receiptBar.Done()
			fatalf("get receipt %d: %v", o.ID, err)
		}
		items, err := ParseReceipt(strings.NewReader(content))
		if err != nil {
			receiptBar.Done()
			fatalf("parse receipt %d: %v", o.ID, err)
		}
		allItems = append(allItems, items...)
		receiptBar.Add(1)
	}
	receiptBar.Done()

	// Classify (API-bound). One bar across all items.
	classifyBar := progress.New("Classifying items", len(allItems))
	ClassifyItems(ctx, client, tree, allItems, func(it *Item) {
		classifyBar.Suffix(it.Name)
		classifyBar.Add(1)
	})
	classifyBar.Done()

	reportUnclassified(allItems)

	fmt.Println()
	fmt.Println("== Spend per category ==")
	fmt.Print(FormatCategoryTree(BuildCategoryReport(allItems)))
}

// monthsBackCutoff returns midnight on the first day of the calendar month
// that is (months-1) months before `now`. So months=1 → first of current
// month; months=2 → first of last month; etc. Local timezone of `now`.
func monthsBackCutoff(now time.Time, months int) time.Time {
	y, m, _ := now.Date()
	return time.Date(y, m-time.Month(months-1), 1, 0, 0, 0, 0, now.Location())
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

// reportUnclassified lists the (deduplicated) item names that fell through
// all classification fallbacks. Printed to stderr so it doesn't pollute the
// aggregations on stdout.
func reportUnclassified(items []*Item) {
	seen := map[string]bool{}
	var names []string
	for _, it := range items {
		if it.MainCategory != unclassified {
			continue
		}
		if seen[it.Name] {
			continue
		}
		seen[it.Name] = true
		names = append(names, it.Name)
	}
	if len(names) == 0 {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, "\n%d unclassified item name(s):\n", len(names))
	for _, n := range names {
		_, _ = fmt.Fprintf(os.Stderr, "  - %s\n", n)
	}
}
