package main

import (
	"fmt"
	"math"
	"parec/rohlik"
	"slices"
	"strings"
)

// colWidth is the name-column width shared by both Bucket and CategoryReport
// rendering, so month rows line up with category-tree rows.
const colWidth = 60

// Bucket is a flat (name, total, %) row. Used by the monthly aggregation.
type Bucket struct {
	Name          string
	TotalDecimals int64
	Percentage    float64
}

func (b *Bucket) String() string {
	return formatRow(b.Name, b.Percentage, b.TotalDecimals)
}

// CategoryReport is a main-category bucket with its subcategory children.
// Percentage is share of the grand total (so children's percentages sum to
// their parent's).
type CategoryReport struct {
	Name          string
	TotalDecimals int64
	Percentage    float64
	Subs          []*CategoryReport
}

// BuildCategoryReport groups items by (MainCategory, SubCategory) and returns
// one entry per main category, with sorted children inside. Both levels are
// sorted by spend descending; ties broken by name.
func BuildCategoryReport(items []*Item) []*CategoryReport {
	type subAcc struct{ total int64 }
	type mainAcc struct {
		total int64
		subs  map[string]*subAcc
	}

	mains := map[string]*mainAcc{}
	var grand int64

	for _, it := range items {
		if it == nil {
			continue
		}
		ma, ok := mains[it.MainCategory]
		if !ok {
			ma = &mainAcc{subs: map[string]*subAcc{}}
			mains[it.MainCategory] = ma
		}
		ma.total += it.PriceDecimals
		grand += it.PriceDecimals

		sa, ok := ma.subs[it.SubCategory]
		if !ok {
			sa = &subAcc{}
			ma.subs[it.SubCategory] = sa
		}
		sa.total += it.PriceDecimals
	}

	out := make([]*CategoryReport, 0, len(mains))
	for name, ma := range mains {
		r := &CategoryReport{
			Name:          name,
			TotalDecimals: ma.total,
			Percentage:    pct(ma.total, grand),
		}
		for sname, sa := range ma.subs {
			r.Subs = append(r.Subs, &CategoryReport{
				Name:          sname,
				TotalDecimals: sa.total,
				Percentage:    pct(sa.total, grand),
			})
		}
		sortReports(r.Subs)
		out = append(out, r)
	}
	sortReports(out)
	return out
}

// FormatCategoryTree renders the report as a tree using box-drawing chars.
// Parent rows are flush left; child rows are prefixed with "├── " or "└── "
// (last child).
func FormatCategoryTree(rs []*CategoryReport) string {
	var sb strings.Builder
	for _, r := range rs {
		sb.WriteString(formatRow(r.Name, r.Percentage, r.TotalDecimals))
		sb.WriteByte('\n')
		for i, sub := range r.Subs {
			prefix := "├── "
			if i == len(r.Subs)-1 {
				prefix = "└── "
			}
			sb.WriteString(formatRow(prefix+sub.Name, sub.Percentage, sub.TotalDecimals))
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// AggregateOrdersByMonth groups order totals by "YYYY-MM" calendar month.
func AggregateOrdersByMonth(orders []rohlik.Order) []*Bucket {
	monthMap := map[string]*Bucket{}
	var out []*Bucket
	var grand int64

	for _, o := range orders {
		t, err := rohlik.ParseOrderTime(o.OrderTime)
		if err != nil {
			continue
		}
		month := t.Format("2006-01")
		b, ok := monthMap[month]
		if !ok {
			b = &Bucket{Name: month}
			monthMap[month] = b
			out = append(out, b)
		}
		// Round-half-away-from-zero: JSON floats like 876.46 are often
		// 876.4599…; raw truncation would yield 87645 instead of 87646.
		amt := int64(math.Round(o.PriceComposition.Total.Amount * 100))
		b.TotalDecimals += amt
		grand += amt
	}

	if grand > 0 {
		for _, b := range out {
			b.Percentage = 100 * float64(b.TotalDecimals) / float64(grand)
		}
	}

	slices.SortFunc(out, func(a, b *Bucket) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// --- shared helpers ---

func sortReports(rs []*CategoryReport) {
	slices.SortFunc(rs, func(a, b *CategoryReport) int {
		switch {
		case a.TotalDecimals > b.TotalDecimals:
			return -1
		case a.TotalDecimals < b.TotalDecimals:
			return 1
		default:
			return strings.Compare(a.Name, b.Name)
		}
	})
}

func pct(part, whole int64) float64 {
	if whole <= 0 {
		return 0
	}
	return 100 * float64(part) / float64(whole)
}

func formatRow(name string, percentage float64, decimals int64) string {
	runes := []rune(name)
	if len(runes) > colWidth {
		runes = append(runes[:colWidth-1], '…')
	}
	pad := colWidth - len(runes)
	if pad < 0 {
		pad = 0
	}
	return fmt.Sprintf("%s%s %6.2f%%  %12s",
		string(runes), strings.Repeat(" ", pad), percentage, formatCZK(decimals))
}

func formatCZK(d int64) string {
	sign := ""
	if d < 0 {
		sign = "-"
		d = -d
	}
	whole := d / 100
	frac := d % 100
	return fmt.Sprintf("%s%d,%02d Kč", sign, whole, frac)
}
