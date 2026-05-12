package main

import (
	"bufio"
	"io"
	"regexp"
	"strings"
	"unicode"
)

// Item is a single purchased line item.
// PriceDecimals is the total line price in minor units (haléř).
type Item struct {
	Name          string `json:"name"`
	PriceDecimals int64  `json:"price"`
	MainCategory  string `json:"mainCategory"`
	SubCategory   string `json:"subCategory"`
}

var (
	reQtyUnitTotal = regexp.MustCompile(
		`^\s*(\d+)\s*[×xX]\s*([0-9][0-9 .]*,[0-9]{2})\s*Kč\s+(-?[0-9][0-9 .]*,[0-9]{2})\s*Kč\s*$`,
	)

	reQtyUnitOnly = regexp.MustCompile(
		`^\s*(\d+)\s*[×xX]\s*([0-9][0-9 .]*,[0-9]{2})\s*Kč\s*$`,
	)

	reWeightTotal = regexp.MustCompile(
		`^\s*([0-9]+(?:[.,][0-9]+)?)\s*kg\s*[×xX]\s*([0-9][0-9 .]*,[0-9]{2})\s*Kč\s*/\s*kg\s+(-?[0-9][0-9 .]*,[0-9]{2})\s*Kč\s*$`,
	)

	reTotalOnly = regexp.MustCompile(
		`^\s*(-?[0-9][0-9 .]*,[0-9]{2})\s*Kč\s*$`,
	)

	reNameQtyUnitTotal = regexp.MustCompile(
		`^\s*(.+?)\s+(\d+)\s*[×xX]\s*([0-9][0-9 .]*,[0-9]{2})\s*Kč\s+(-?[0-9][0-9 .]*,[0-9]{2})\s*Kč\s*$`,
	)

	reNamePriceOnly = regexp.MustCompile(
		`^\s*(.+?)\s+(-?[0-9][0-9 .]*,[0-9]{2})\s*Kč\s*$`,
	)

	reWeightOnly = regexp.MustCompile(
		`^\s*[0-9]+(?:[.,][0-9]+)?\s*kg\s*$`,
	)
)

// ParseReceipt parses a plaintext receipt and returns the purchased items.
//
// The parser is line-oriented and section-gated: only lines between a "Zboží"
// start marker and an end marker ("Doprava a platba", "Cena celkem",
// "Způsob úhrady", "Zákazník", or "Dodavatel") are considered items.
// Defensive about line wraps from pdftotext: name lines accumulate until a
// price line flushes them; weighed items can emit several consecutive price
// lines under one name.
func ParseReceipt(r io.Reader) ([]*Item, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return parseText(string(b))
}

type receiptParser struct {
	items                   []*Item
	inItems                 bool
	pendingNameLines        []string
	awaitingStandaloneTotal bool
	lastItemName            string
	allowReuseLastName      bool
}

func (p *receiptParser) resetPending() {
	p.pendingNameLines = nil
	p.awaitingStandaloneTotal = false
	p.allowReuseLastName = false
}

func (p *receiptParser) pendingName() string {
	return cleanItemName(strings.Join(p.pendingNameLines, " "))
}

func (p *receiptParser) emit(name string, priceDecimals int64) {
	p.items = append(p.items, &Item{Name: name, PriceDecimals: priceDecimals})
	p.lastItemName = name
	p.allowReuseLastName = true
	p.pendingNameLines = nil
	p.awaitingStandaloneTotal = false
}

func (p *receiptParser) emitExplicitName(name string, priceDecimals int64) {
	name = cleanItemName(name)
	if !isValidItemName(name) {
		return
	}
	p.emit(name, priceDecimals)
}

func (p *receiptParser) emitPendingOrLast(priceDecimals int64) {
	name := p.pendingName()
	if name == "" && p.allowReuseLastName {
		name = p.lastItemName
	}
	if !isValidItemName(name) {
		p.pendingNameLines = nil
		p.allowReuseLastName = false
		p.awaitingStandaloneTotal = false
		return
	}
	p.emit(name, priceDecimals)
}

func parseText(text string) ([]*Item, error) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	p := &receiptParser{}

	for scanner.Scan() {
		line := normalizeLine(scanner.Text())
		if line == "" {
			p.allowReuseLastName = false
			continue
		}

		if isStartOfItemsSection(line) {
			p.inItems = true
			p.resetPending()
			continue
		}
		if isEndOfItemsSection(line) {
			p.inItems = false
			p.resetPending()
			continue
		}

		if !p.inItems {
			continue
		}

		if isNonItemHeader(line) || isProbablySupplierLine(line) || isReturnGoods(line) {
			p.resetPending()
			continue
		}

		if reWeightOnly.MatchString(line) {
			continue
		}

		if m := reNameQtyUnitTotal.FindStringSubmatch(line); m != nil {
			if price, ok := parseCZKToDecimals(m[4]); ok {
				p.emitExplicitName(m[1], price)
			}
			continue
		}

		if p.awaitingStandaloneTotal {
			if m := reTotalOnly.FindStringSubmatch(line); m != nil {
				if price, ok := parseCZKToDecimals(m[1]); ok {
					p.emitPendingOrLast(price)
					continue
				}
				p.pendingNameLines = nil
				p.awaitingStandaloneTotal = false
				continue
			}
			p.pendingNameLines = nil
			p.awaitingStandaloneTotal = false
		}

		if m := reQtyUnitTotal.FindStringSubmatch(line); m != nil {
			if price, ok := parseCZKToDecimals(m[3]); ok {
				p.emitPendingOrLast(price)
			}
			continue
		}

		if m := reWeightTotal.FindStringSubmatch(line); m != nil {
			if price, ok := parseCZKToDecimals(m[3]); ok {
				p.emitPendingOrLast(price)
			}
			continue
		}

		if reQtyUnitOnly.MatchString(line) {
			p.awaitingStandaloneTotal = true
			p.allowReuseLastName = false
			continue
		}

		if m := reTotalOnly.FindStringSubmatch(line); m != nil {
			if price, ok := parseCZKToDecimals(m[1]); ok {
				p.emitPendingOrLast(price)
			}
			continue
		}

		if m := reNamePriceOnly.FindStringSubmatch(line); m != nil {
			name := cleanItemName(m[1])
			if isValidItemName(name) && !isNonItemSummaryLine(name) {
				if price, ok := parseCZKToDecimals(m[2]); ok {
					p.emit(name, price)
				}
			}
			continue
		}

		p.allowReuseLastName = false
		p.pendingNameLines = append(p.pendingNameLines, line)
	}

	if err := scanner.Err(); err != nil {
		return p.items, err
	}
	return p.items, nil
}

func normalizeLine(s string) string {
	s = strings.ReplaceAll(s, " ", " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return strings.Join(strings.Fields(s), " ")
}

func cleanItemName(s string) string {
	s = normalizeLine(s)
	s = strings.Trim(s, "-–—")
	return s
}

func parseCZKToDecimals(amount string) (int64, bool) {
	s := strings.TrimSpace(amount)
	s = strings.ReplaceAll(s, "Kč", "")
	s = strings.ReplaceAll(s, " ", " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}

	sign := int64(1)
	if strings.HasPrefix(s, "-") {
		sign = -1
		s = strings.TrimSpace(strings.TrimPrefix(s, "-"))
	}

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == ',' || unicode.IsDigit(r):
			b.WriteRune(r)
		case unicode.IsSpace(r) || r == '.':
		default:
		}
	}
	s = b.String()

	parts := strings.SplitN(s, ",", 2)
	if len(parts) != 2 {
		return 0, false
	}
	wholeStr, fracStr := parts[0], parts[1]
	if wholeStr == "" {
		wholeStr = "0"
	}
	if len(fracStr) == 1 {
		fracStr += "0"
	}
	if len(fracStr) != 2 {
		return 0, false
	}

	var whole int64
	for _, r := range wholeStr {
		if r < '0' || r > '9' {
			return 0, false
		}
		whole = whole*10 + int64(r-'0')
	}
	var frac int64
	for _, r := range fracStr {
		if r < '0' || r > '9' {
			return 0, false
		}
		frac = frac*10 + int64(r-'0')
	}
	return sign * (whole*100 + frac), true
}

func isStartOfItemsSection(line string) bool {
	l := strings.ToLower(line)
	return l == "zboží" || l == "zbozi" || l == "doručené položky" || l == "doručené polozky"
}

func isEndOfItemsSection(line string) bool {
	l := strings.ToLower(line)
	switch {
	case strings.HasPrefix(l, "doprava a platba"):
		return true
	case strings.HasPrefix(l, "cena celkem"):
		return true
	case strings.HasPrefix(l, "způsob úhrady"), strings.HasPrefix(l, "zpusob uhrady"):
		return true
	case strings.HasPrefix(l, "zákazník"), strings.HasPrefix(l, "zakaznik"):
		return true
	case strings.HasPrefix(l, "dodavatel"):
		return true
	default:
		return false
	}
}

func isNonItemHeader(line string) bool {
	l := strings.ToLower(line)
	if l == "dodací list" || l == "dodaci list" {
		return true
	}
	if strings.HasPrefix(l, "objednávka") || strings.HasPrefix(l, "objednavka") {
		return true
	}
	if strings.HasPrefix(l, "doprava a platba") {
		return true
	}

	allSep := true
	stars := 0
	for _, r := range line {
		if r == '*' {
			stars++
			continue
		}
		if unicode.IsSpace(r) {
			continue
		}
		allSep = false
		break
	}
	return allSep && stars >= 3
}

func isProbablySupplierLine(line string) bool {
	l := strings.ToLower(line)
	if strings.Contains(l, "kč") || strings.Contains(l, "×") {
		return false
	}
	corp := []string{" s.r.o.", " a.s.", " spol.", " ltd", " gmbh", " inc"}
	for _, c := range corp {
		if strings.Contains(l, c) {
			return true
		}
	}
	return false
}

func isReturnGoods(line string) bool {
	return strings.Contains(line, "vratné obaly")
}

func isValidItemName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if isNonItemSummaryLine(name) {
		return false
	}
	for _, r := range name {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func isNonItemSummaryLine(s string) bool {
	l := strings.ToLower(strings.TrimSpace(s))
	denyPrefixes := []string{
		"doprava",
		"spropitné", "spropitne",
		"sleva",
		"cena celkem",
		"zákazník", "zakaznik",
		"dodavatel",
		"způsob úhrady", "zpusob uhrady",
		"zákazník platí", "zakaznik plati",
		"bankovní spojení", "bankovni spojeni",
		"spisová značka", "spisova znacka",
		"bio certifikace",
		"doručit do", "dorucit do",
		"ičo", "ico",
		"dič", "dic",
		"zboží ", "zbozi ",
	}
	for _, p := range denyPrefixes {
		if strings.HasPrefix(l, p) {
			return true
		}
	}
	return false
}
