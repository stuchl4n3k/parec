package rohlik

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Category is a Rohlik category result from autocomplete.
type Category struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	TopParentName string `json:"topParentName"`
	Link          string `json:"link"`
	NameLong      string `json:"nameLong"`
	Type          string `json:"type"`
}

// AutocompleteResult carries productIds + categories from a single query.
type AutocompleteResult struct {
	ProductIDs []int64    `json:"productIds"`
	Categories []Category `json:"categories"`
}

// AutocompleteFull runs Rohlik autocomplete for a free-text query and returns
// the matched productIds + categories. Results are cached on disk under
// "autocomplete-full|<normalized query>". The first uncached call sleeps
// 1–2s on exit to be polite to the unofficial endpoint.
func (c *Client) AutocompleteFull(ctx context.Context, query string) (*AutocompleteResult, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, errors.New("query cannot be empty")
	}

	cacheKey := c.cacheKey("autocomplete-full", q)

	var cached AutocompleteResult
	if c.cache.Get(cacheKey, &cached) {
		return &AutocompleteResult{
			ProductIDs: append([]int64(nil), cached.ProductIDs...),
			Categories: append([]Category(nil), cached.Categories...),
		}, nil
	}

	apiURL := fmt.Sprintf("%s/frontend-service/autocomplete?referer=whisperer&companyId=1", c.BaseURL)
	u, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	qs := u.Query()
	qs.Set("search", q)
	u.RawQuery = qs.Encode()

	req, err := c.newRequest(ctx, http.MethodGet, u.String(), nil)
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

	var decoded struct {
		ProductIDs []int64    `json:"productIds"`
		Categories []Category `json:"categories"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}

	time.Sleep(time.Duration(rand.IntN(1000)) * time.Millisecond)

	result := AutocompleteResult{ProductIDs: decoded.ProductIDs, Categories: decoded.Categories}
	if err := c.cache.Put(cacheKey, result); err != nil {
		return &result, fmt.Errorf("fetched OK but failed to cache: %w", err)
	}
	return &result, nil
}

// cacheKey normalizes (action,query) into a stable on-disk cache key.
// Trim, collapse whitespace, lowercase. Unicode-aware.
func (c *Client) cacheKey(action, query string) string {
	q := strings.TrimSpace(action + "|" + query)
	if q == "" {
		return ""
	}
	q = strings.Join(strings.Fields(q), " ")
	q = strings.ToLower(q)
	return q
}
