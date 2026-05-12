package rohlik

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"parec/pdftotext"
	"strings"
)

func (c *Client) GetOrderReceipt(ctx context.Context, id int64) (string, error) {
	apiURL := fmt.Sprintf("%s/frontend-service/files/receipt/%d", c.BaseURL, id)
	req, err := c.newRequest(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
		return "", fmt.Errorf("receipt %d: unexpected status %d: %s",
			id, resp.StatusCode, strings.TrimSpace(string(b)))
	}

	pdfData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return pdftotext.Extract(pdfData)
}
