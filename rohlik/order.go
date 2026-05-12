package rohlik

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Order struct {
	ID               int64            `json:"id"`
	ItemsCount       int              `json:"itemsCount"`
	PriceComposition PriceComposition `json:"priceComposition"`
	OrderTime        string           `json:"orderTime"`
}

type PriceComposition struct {
	Total Price `json:"total"`
}

type Price struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// orderTimeLayout matches Rohlik's OrderTime format ("2026-05-07T20:18:34.000+0200").
const orderTimeLayout = "2006-01-02T15:04:05.000-0700"

// ParseOrderTime parses Rohlik's OrderTime string.
func ParseOrderTime(s string) (time.Time, error) {
	return time.Parse(orderTimeLayout, s)
}

// GetOrdersSince paginates /api/v3/orders/delivered and returns every order
// whose OrderTime is at or after cutoff. The upstream feed is ordered
// newest-first, so we stop as soon as we encounter an older order or run
// out of pages.
func (c *Client) GetOrdersSince(ctx context.Context, cutoff time.Time) ([]Order, error) {
	const pageSize = 20
	var all []Order
	offset := 0
	for {
		page, err := c.fetchOrdersPage(ctx, offset, pageSize)
		if err != nil {
			return all, err
		}
		if len(page) == 0 {
			break
		}

		olderSeen := false
		for _, o := range page {
			t, err := ParseOrderTime(o.OrderTime)
			if err != nil {
				continue
			}
			if t.Before(cutoff) {
				olderSeen = true
				break
			}
			all = append(all, o)
		}
		if olderSeen || len(page) < pageSize {
			break
		}
		offset += pageSize
	}
	return all, nil
}

func (c *Client) fetchOrdersPage(ctx context.Context, offset, limit int) ([]Order, error) {
	url := fmt.Sprintf("https://www.rohlik.cz/api/v3/orders/delivered?offset=%d&limit=%d", offset, limit)
	req, err := c.newRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
		return nil, fmt.Errorf("orders page offset=%d: unexpected status %d: %s",
			offset, resp.StatusCode, strings.TrimSpace(string(b)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	page := make([]Order, 0, limit)
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, err
	}
	return page, nil
}
