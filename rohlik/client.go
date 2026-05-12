package rohlik

import (
	"context"
	"errors"
	"github.com/chromedp/cdproto/network"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	// Headless controls whether the chromedp login flow runs without a
	// visible browser window. Default true — flip to false locally if you
	// need to watch the flow for debugging or hit a CAPTCHA manually.
	Headless  = true
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
		"AppleWebKit/537.36 (KHTML, like Gecko) " +
		"Chrome/122.0.0.0 Safari/537.36"
	AcceptLanguage = "cs-CZ,cs;q=0.9,en;q=0.8"

	StartURL   = "https://www.rohlik.cz/"
	CookieJar  = "./data/cookies.txt"
	TimeoutStr = "20s"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client

	// Headers (customizable):
	UserAgent      string
	AcceptLanguage string
	RefererHeader  string
	XOrigin        string
	AltUsed        string

	// State:
	authenticated bool
	cookies       []*network.Cookie
	cache         *FileCache
}

// NewClient creates a client and loads/initializes a file-backed cache.
// cacheCSVPath can be e.g. "./rohlik_cache.csv".
func NewClient(cachePath string) (*Client, error) {
	if strings.TrimSpace(cachePath) == "" {
		return nil, errors.New("cachePath is required")
	}

	c := &Client{
		BaseURL: "https://www.rohlik.cz/services",
		HTTP:    &http.Client{Timeout: 15 * time.Second},

		UserAgent:      "Mozilla/5.0 (X11; Linux x86_64; rv:146.0) Gecko/20100101 Firefox/146.0",
		AcceptLanguage: AcceptLanguage,
		RefererHeader:  "https://www.rohlik.cz/",
		XOrigin:        "WEB",
		AltUsed:        "www.rohlik.cz",
	}

	fc, err := NewFileCache(cachePath)
	if err != nil {
		return nil, err
	}
	c.cache = fc

	return c, nil
}

func (c *Client) newRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", c.AcceptLanguage)
	req.Header.Set("Referer", c.RefererHeader)
	req.Header.Set("X-Origin", c.XOrigin)
	req.Header.Set("Alt-Used", c.AltUsed)

	for _, c := range c.cookies {
		// Only send Rohlik cookies.
		if strings.Contains(strings.ToLower(c.Domain), "rohlik.cz") {
			req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value, HttpOnly: c.HTTPOnly, Secure: c.Secure, Domain: c.Domain})
		}
	}
	return req, nil
}
