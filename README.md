# parec

**PA**rser of **REC**eipts — a personal Go CLI that pulls your
delivered-order history from [Rohlik.cz](https://www.rohlik.cz), parses each
PDF receipt, classifies every line item against Rohlik's own category tree,
and prints a spend breakdown.

For each item, parec queries Rohlik **autocomplete** and combines two signals:

- `topParentName` → the **main** category (e.g. "Ovoce a zelenina") — delivered
  directly on every autocomplete result.
- The depth-1 ancestor of the matched leaf in a cached navigation tree (built
  from the v5/v4 navigation endpoints) → the **sub** category. If the tree
  doesn't know the leaf, falls back to the leaf name itself so the report
  still shows something informative.

Output: spend per month, plus a tree of spend per main / sub category.

## Requirements

- **Go 1.24+**
- **`pdftotext`** on `$PATH` (Debian/Ubuntu: `apt install poppler-utils`)
- A graphical environment for the initial login — the underlying `chromedp`
  flow runs headful by default. Set `rohlik.Headless = true` to override.
  If Rohlik throws CAPTCHA/2FA, you'll need to solve it once in the visible
  window; cookies are then reused.
- A Rohlik.cz account.

## Quickstart

```bash
export ROHLIK_EMAIL='you@example.com'
export ROHLIK_PASSWORD='...'

make run                       # current calendar month only (default --months=1)
go run . --months=2             # current month + previous month
go run . --months=12            # rolling 12 calendar months
```

`--months=N` includes the current calendar month and the N−1 previous ones,
counting back from today. The cutoff is midnight on the first day of the
oldest month — on May 11, `--months=2` covers everything from April 1 onward.

First run builds the Rohlik category tree (up to 5 minutes, indeterminate progress
bar) and triggers one autocomplete call per uncached item (~1–2 s each).
Subsequent runs read both from the on-disk cache and are near-instant.

## Output

```
Downloading receipts [############################] 100% 20/20 (15s)
Classifying items    [############################] 100% 643/643 (2m18s)

== Spend per month ==
2026-03                                                       45.12%   12 345,67 Kč
2026-04                                                       54.88%   15 000,11 Kč

== Spend per category ==
Mléčné a chlazené                                             22.40%    6 110,40 Kč
├── Sýry                                                       8.20%    2 236,80 Kč
├── Mléko a smetana                                            8.10%    2 211,30 Kč
└── Jogurty                                                    6.10%    1 662,30 Kč
Ovoce a zelenina                                              17.95%    4 896,75 Kč
├── Čerstvá zelenina                                           6.42%    1 752,90 Kč
└── Čerstvé ovoce                                             11.53%    3 143,85 Kč
Trvanlivé potraviny                                           14.20%    3 873,82 Kč
├── Káva, čaj, kakao                                           5.30%    1 446,18 Kč
└── ...
```

Sub-percentages are share of the **grand total** (so each main's children sum
to its own row).

## Data and caches

All under `data/`:

- `data/cookies.txt` — chromedp cookie jar. Delete to force a fresh login.
- `data/.cache.json` — JSON store for autocomplete results. Delete to re-query everything from scratch.
- `data/sample*.txt` — parser test fixtures.

## Make targets

| target       | what it does                      |
|--------------|-----------------------------------|
| `make build` | build the binary into `bin/parec` |
| `make run`   | `go run .`                        |
| `make test`  | `go test ./...`                   |
| `make vet`   | `go vet ./...`                    |
| `make fmt`   | `gofmt -w .`                      |
| `make tidy`  | `go mod tidy`                     |
| `make clean` | remove `bin/`                     |

## Layout

```
parec/
├── main.go          -- entry: auth → tree → orders → receipts → classify → aggregate
├── parser.go        -- receipt-text parser (Czech, line-oriented state machine)
├── classifier.go    -- per-item lookup against Rohlik APIs + nav tree
├── aggregator.go    -- spend by month / main / (main, sub)
├── parser_test.go   -- golden tests against data/sample*.txt
├── progress/        -- tiny terminal progress-bar package
├── rohlik/          -- HTTP client, chromedp login, API methods, on-disk cache
├── pdftotext/       -- thin shell-out to the `pdftotext` binary
├── data/            -- runtime data and test fixtures
├── API.md           -- raw Rohlik endpoints reference
├── Makefile
└── README.md
```

## Notes

- **Unofficial API.** Rohlik may rate-limit, block, or change endpoints
  without notice. Autocomplete, product, and navigation fetchers each sleep
  ~0.5–2s between uncached calls; play fair, do not remove those sleeps.
- **Money is integer minor units** (`int64`, haléř) throughout.
- **Personal data.** Cookies, the on-disk cache (which records your queries),
  and real receipts under `data/` are personal.
- **No warranty, no affiliation** with Rohlik / Velká Pecka s.r.o.
