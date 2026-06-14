// Package sources fetches the FREE market data the default brain reasons over:
// recent prices (Yahoo) and recent headlines (Google News RSS). Both are
// behind interfaces so users can swap in their own providers without forking.
package sources

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var httpc = &http.Client{Timeout: 20 * time.Second}

type Quote struct {
	Symbol    string
	Price     float64
	PrevClose float64
	ChangePct float64
	Low5d     float64
	High5d    float64
}

type Headline struct {
	Title  string
	Source string
}

// PriceSource and NewsSource are the swap points for custom data feeds.
type PriceSource interface {
	Quote(symbol string) (Quote, error)
}
type NewsSource interface {
	Headlines(symbol string, max int) ([]Headline, error)
}

// ---- free default implementations ----

type Yahoo struct{}

func (Yahoo) Quote(symbol string) (Quote, error) {
	u := "https://query1.finance.yahoo.com/v8/finance/chart/" + url.PathEscape(symbol) + "?range=5d&interval=1d"
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := httpc.Do(req)
	if err != nil {
		return Quote{}, err
	}
	defer resp.Body.Close()
	var out struct {
		Chart struct {
			Result []struct {
				Meta struct {
					RegularMarketPrice float64 `json:"regularMarketPrice"`
					ChartPreviousClose float64 `json:"chartPreviousClose"`
				} `json:"meta"`
				Indicators struct {
					Quote []struct {
						Close []*float64 `json:"close"`
					} `json:"quote"`
				} `json:"indicators"`
			} `json:"result"`
		} `json:"chart"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Quote{}, err
	}
	if len(out.Chart.Result) == 0 {
		return Quote{}, fmt.Errorf("no data for %s", symbol)
	}
	r := out.Chart.Result[0]
	q := Quote{Symbol: symbol, Price: r.Meta.RegularMarketPrice, PrevClose: r.Meta.ChartPreviousClose}
	var closes []float64
	if len(r.Indicators.Quote) > 0 {
		for _, c := range r.Indicators.Quote[0].Close {
			if c != nil {
				closes = append(closes, *c)
			}
		}
	}
	if len(closes) >= 2 {
		q.Low5d, q.High5d = closes[0], closes[0]
		for _, c := range closes {
			if c < q.Low5d {
				q.Low5d = c
			}
			if c > q.High5d {
				q.High5d = c
			}
		}
		prev := closes[len(closes)-2]
		if q.PrevClose == 0 {
			q.PrevClose = prev
		}
	}
	if q.PrevClose > 0 {
		q.ChangePct = (q.Price/q.PrevClose - 1) * 100
	}
	return q, nil
}

type GoogleNews struct{}

func (GoogleNews) Headlines(symbol string, max int) ([]Headline, error) {
	q := url.QueryEscape(symbol + " stock")
	u := "https://news.google.com/rss/search?q=" + q + "&hl=en-US&gl=US&ceid=US:en"
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var feed struct {
		Channel struct {
			Items []struct {
				Title  string `xml:"title"`
				Source string `xml:"source"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, err
	}
	var hs []Headline
	for _, it := range feed.Channel.Items {
		hs = append(hs, Headline{Title: it.Title, Source: it.Source})
		if len(hs) >= max {
			break
		}
	}
	return hs, nil
}
