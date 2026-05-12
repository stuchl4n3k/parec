// rohlik_login.go
//
// Usage examples:
//   ROHLIK_EMAIL="you@example.com" ROHLIK_PASSWORD="secret" go run . --cookie-file cookies.json
//   go run . --email you@example.com --password secret --headless=true --timeout=45s --cookie-file cookies.json
//
// Notes:
// - This drives the real Rohlik.cz website UI, so selectors/flows can change.
// - If Rohlik shows CAPTCHA/2FA, you must handle that manually (e.g., run headful and pause).

package rohlik

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/chromedp/cdproto/cdp"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const (
	// Broad selectors to survive minor UI changes.
	emailInputSelector = `input[type="email"], input[autocomplete="email"], input[autocomplete="username"], input[name*="mail" i]`
	passInputSelector  = `input[type="password"], input[autocomplete="current-password"], input[name*="pass" i]`
)

type loginResult struct {
	URL      string `json:"url"`
	LoggedIn bool   `json:"loggedIn"`
}

func (c *Client) Auth() (bool, []*network.Cookie, error) {
	timeout, err := time.ParseDuration(TimeoutStr)
	if err != nil {
		return false, nil, fmt.Errorf("invalid TimeoutStr %q: %w", TimeoutStr, err)
	}

	// Root context with timeout.
	rootCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Chrome allocator options.
	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", Headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("user-agent", UserAgent),
		chromedp.Flag("lang", AcceptLanguage),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(rootCtx, allocOpts...)
	defer allocCancel()

	ctx, ctxCancel := chromedp.NewContext(allocCtx,
		chromedp.WithLogf(log.Printf),
		chromedp.WithErrorf(filteredErrorf),
	)
	defer ctxCancel()

	if CookieJar != "" {
		c.cookies, err = readCookiesJSON(CookieJar)
		if err != nil {
			log.Printf("failed to read cookies: %v", err)
		}
	}

	var loginRes loginResult

	// Run auth check flow.
	if err := chromedp.Run(ctx,
		network.Enable(),
		setCookies(c.cookies),

		chromedp.Navigate(StartURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(1200*time.Millisecond),
		detectLoginState(&loginRes),
	); err != nil {
		return false, nil, fmt.Errorf("login flow failed: %v", err)
	}

	if !loginRes.LoggedIn {
		// Run login flow.
		email := os.Getenv("ROHLIK_EMAIL")
		password := os.Getenv("ROHLIK_PASSWORD")
		if email == "" || password == "" {
			return false, nil, fmt.Errorf("missing environment variable ROHLIK_EMAIL/ROHLIK_PASSWORD")
		}

		if err := chromedp.Run(ctx,
			// Open login UI (best-effort).
			openLoginUIBestEffort(),
			chromedp.Sleep(800*time.Millisecond),

			// Wait for email/password inputs to be visible/ready.
			chromedp.WaitVisible(emailInputSelector, chromedp.ByQuery),
			chromedp.WaitVisible(passInputSelector, chromedp.ByQuery),

			// Fill and submit.
			fillAndSubmit(email, password),

			// Allow UI to update / redirect.
			chromedp.Sleep(2500*time.Millisecond),
			detectLoginState(&loginRes),
		); err != nil {
			return false, nil, fmt.Errorf("login flow failed: %v", err)
		}
	}

	// Fetch cookies.
	chromeCtx := chromedp.FromContext(ctx)
	cookies, err := network.GetCookies().Do(cdp.WithExecutor(ctx, chromeCtx.Target))
	if err != nil {
		return false, nil, fmt.Errorf("failed to read cookies: %v", err)
	}

	// Print a summary.
	fmt.Println("== Rohlik.cz login attempt summary ==")
	fmt.Printf("URL: %s\n", loginRes.URL)
	fmt.Printf("Logged in: %v\n", loginRes.LoggedIn)
	fmt.Printf("Cookies captured: %d\n", len(cookies))

	// Write cookies to file if requested (includes values). A write failure
	// here must NOT discard a successful login — log it and continue.
	if CookieJar != "" {
		if err := writeCookiesJSON(CookieJar, cookies); err != nil {
			log.Printf("failed to write cookies to %s: %v (continuing anyway)", CookieJar, err)
		} else {
			fmt.Printf("\nWrote cookies JSON to: %s\n", CookieJar)
		}
	}

	c.authenticated = loginRes.LoggedIn

	return c.authenticated, cookies, nil
}

// openLoginUIBestEffort clicks the "Přihlásit se" / "Log in" entry if present.
// It never errors if it can't find it (but the flow likely won't proceed).
func openLoginUIBestEffort() chromedp.Action {
	js := `(() => {
		const rx = /(přihlásit se|přihlášení|log in|sign in)/i;
		const el = Array.from(document.querySelectorAll("a,button,[role='button']"))
			.find(e => e && e.textContent && rx.test(e.textContent.trim()));
		if (el) { try { el.click(); return true; } catch (e) { return false; } }
		return false;
	})()`
	return chromedp.Evaluate(js, nil)
}

// fillAndSubmit sets input values in a framework-friendly way and submits the nearest form / submit button.
func fillAndSubmit(email, password string) chromedp.Action {
	js := fmt.Sprintf(`(() => {
		const email = %s;
		const password = %s;

		const emailSel = %s;
		const passSel  = %s;

		const emailEl = document.querySelector(emailSel);
		const passEl  = document.querySelector(passSel);

		if (!emailEl || !passEl) {
			return { ok:false, reason:"inputs not found" };
		}

		const setNativeValue = (el, value) => {
			const proto = Object.getPrototypeOf(el);
			const desc = Object.getOwnPropertyDescriptor(proto, "value");
			if (desc && desc.set) desc.set.call(el, value);
			else el.value = value;

			el.dispatchEvent(new Event("input",  { bubbles:true }));
			el.dispatchEvent(new Event("change", { bubbles:true }));
		};

		emailEl.focus();
		setNativeValue(emailEl, email);

		passEl.focus();
		setNativeValue(passEl, password);

		const form = passEl.form || emailEl.form;
		let submit = null;

		if (form) {
			submit = form.querySelector("button[type='submit'], input[type='submit']");
		}
		if (!submit) {
			submit = document.querySelector("button[type='submit'], input[type='submit']");
		}
		if (submit) {
			submit.click();
			return { ok:true, submitted:"button" };
		}

		// Last resort: try form submit.
		if (form) {
			try { form.requestSubmit ? form.requestSubmit() : form.submit(); return { ok:true, submitted:"form" }; } catch (e) {}
		}
		return { ok:false, reason:"no submit found" };
	})()`,
		strconv.Quote(email),
		strconv.Quote(password),
		strconv.Quote(emailInputSelector),
		strconv.Quote(passInputSelector),
	)
	return chromedp.Evaluate(js, nil)
}

func detectLoginState(out *loginResult) chromedp.Action {
	js := `(() => {
		const url = location.href;
		const loggedIn = document.querySelector("#headerUser") !== null;
		return { url, loggedIn };
	})()`
	return chromedp.Evaluate(js, out)
}

func setCookies(cookies []*network.Cookie) chromedp.Action {
	if cookies == nil {
		cookies = []*network.Cookie{}
	}
	return chromedp.ActionFunc(func(ctx context.Context) error {
		for _, cookie := range cookies {
			if strings.Contains(cookie.Value, "\"") {
				continue
			}
			setter := network.SetCookie(cookie.Name, cookie.Value).
				WithDomain(cookie.Domain).
				WithPath(cookie.Path).
				WithHTTPOnly(cookie.HTTPOnly).
				WithSecure(cookie.Secure)
			// Only attach Expires for persistent cookies. CDP treats a
			// non-positive Expires as "expired now" — applying it to a
			// session cookie (Expires==0) instantly invalidates it.
			if cookie.Expires > 0 {
				expr := cdp.TimeSinceEpoch(time.Unix(int64(cookie.Expires), 0))
				setter = setter.WithExpires(&expr)
			}
			if err := setter.Do(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

// LoadCookies reads the cookie jar from disk into the client without opening
// chromedp. Useful for code paths (and tests) that want to inherit the
// existing Cloudflare clearance + session cookies from a prior Auth() call.
// No-op if the cookie file is missing.
func (c *Client) LoadCookies() error {
	if CookieJar == "" {
		return nil
	}
	cookies, err := readCookiesJSON(CookieJar)
	if err != nil {
		return err
	}
	c.cookies = cookies
	return nil
}

func readCookiesJSON(path string) ([]*network.Cookie, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	cookies := make([]*network.Cookie, 0)
	if err := json.Unmarshal(content, &cookies); err != nil {
		return nil, err
	}

	return cookies, nil
}

func writeCookiesJSON(path string, cookies []*network.Cookie) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(cookies)
}

// filteredErrorf is the chromedp error logger. It drops the noisy
// "unhandled <domain> event: *<type>" lines that chromedp emits when the
// pinned `cdproto` package doesn't have a Go type for a CDP event Chrome
// sends (e.g. dom.EventTopLayerElementsUpdated on newer Chrome builds).
// These are harmless — chromedp just can't decode the event payload — and
// they pollute startup output. Everything else is forwarded to log.Printf.
func filteredErrorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if strings.Contains(msg, "unhandled") && strings.Contains(msg, "event") {
		return
	}
	log.Printf(format, args...)
}
