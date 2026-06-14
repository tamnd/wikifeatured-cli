// Package wikifeatured is the library behind the wikifeatured command line:
// the HTTP client, request shaping, and the typed data models for the
// Wikipedia Featured Content REST API (https://en.wikipedia.org/api/rest_v1/feed/).
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public API throws under load.
package wikifeatured

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Host is the Wikipedia API host this client talks to.
const Host = "en.wikipedia.org"

// baseURL is the root every request is built from.
const baseURL = "https://en.wikipedia.org/api/rest_v1"

// DefaultUserAgent identifies the client to Wikipedia.
const DefaultUserAgent = "wikifeatured-cli/0.1 (tamnd87@gmail.com)"

// Client talks to the Wikipedia Featured Content REST API over HTTP.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	BaseURL   string
	// Rate is the minimum gap between requests. Zero means no pacing.
	Rate    time.Duration
	Retries int

	last time.Time
}

// NewClient returns a Client with sensible defaults.
func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 15 * time.Second},
		UserAgent: DefaultUserAgent,
		BaseURL:   baseURL,
		Rate:      500 * time.Millisecond,
		Retries:   3,
	}
}

// Featured is the today's featured article record.
type Featured struct {
	Title   string `json:"title" kit:"id"`
	Extract string `json:"extract"`
	Date    string `json:"date"`
}

// Article is one entry in the most-read list.
type Article struct {
	Rank    int    `json:"rank" kit:"id"`
	Title   string `json:"title"`
	Views   int    `json:"views"`
	Extract string `json:"extract"`
}

// NewsItem is one news story from Wikipedia's current events.
type NewsItem struct {
	Story     string `json:"story" kit:"id"`
	LinkCount int    `json:"link_count"`
}

// Event is a historical event from the on-this-day feed.
type Event struct {
	Year int    `json:"year" kit:"id"`
	Text string `json:"text"`
}

// --- raw response shapes ---

type featuredResp struct {
	TFA struct {
		Title   string `json:"title"`
		Extract string `json:"extract"`
	} `json:"tfa"`
	MostRead struct {
		Articles []struct {
			Rank    int    `json:"rank"`
			Title   string `json:"title"`
			Views   int    `json:"views"`
			Extract string `json:"extract"`
		} `json:"articles"`
	} `json:"mostread"`
	News []struct {
		Story string `json:"story"`
		Links []struct {
			Title string `json:"title"`
		} `json:"links"`
	} `json:"news"`
}

type onThisDayResp struct {
	Events []struct {
		Year int    `json:"year"`
		Text string `json:"text"`
	} `json:"events"`
}

// --- API methods ---

// GetFeatured fetches the featured article for the given date (YYYY/MM/DD).
func (c *Client) GetFeatured(ctx context.Context, date string) (*Featured, error) {
	url := c.BaseURL + "/feed/featured/" + date
	body, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	var resp featuredResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode featured: %w", err)
	}
	extract := resp.TFA.Extract
	if len(extract) > 500 {
		extract = extract[:500]
	}
	return &Featured{
		Title:   resp.TFA.Title,
		Extract: extract,
		Date:    date,
	}, nil
}

// GetMostRead fetches the most-read articles for the given date (YYYY/MM/DD).
func (c *Client) GetMostRead(ctx context.Context, date string) ([]*Article, error) {
	url := c.BaseURL + "/feed/featured/" + date
	body, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	var resp featuredResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode mostread: %w", err)
	}
	out := make([]*Article, 0, len(resp.MostRead.Articles))
	for _, a := range resp.MostRead.Articles {
		extract := a.Extract
		if len(extract) > 200 {
			extract = extract[:200]
		}
		out = append(out, &Article{
			Rank:    a.Rank,
			Title:   a.Title,
			Views:   a.Views,
			Extract: extract,
		})
	}
	return out, nil
}

var htmlTagRE = regexp.MustCompile(`<[^>]+>`)

// GetNews fetches current news items for the given date (YYYY/MM/DD).
func (c *Client) GetNews(ctx context.Context, date string) ([]*NewsItem, error) {
	url := c.BaseURL + "/feed/featured/" + date
	body, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	var resp featuredResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode news: %w", err)
	}
	out := make([]*NewsItem, 0, len(resp.News))
	for _, n := range resp.News {
		story := strings.TrimSpace(htmlTagRE.ReplaceAllString(n.Story, ""))
		out = append(out, &NewsItem{
			Story:     story,
			LinkCount: len(n.Links),
		})
	}
	return out, nil
}

// GetOnThisDay fetches historical events for the given month/day (MM/DD).
func (c *Client) GetOnThisDay(ctx context.Context, date string) ([]*Event, error) {
	url := c.BaseURL + "/feed/onthisday/events/" + date
	body, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	var resp onThisDayResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode onthisday: %w", err)
	}
	out := make([]*Event, 0, len(resp.Events))
	for _, e := range resp.Events {
		out = append(out, &Event{
			Year: e.Year,
			Text: e.Text,
		})
	}
	return out, nil
}

// --- HTTP transport ---

// Get fetches a URL and returns the body. It paces and retries according to
// the client's settings.
func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, url string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.Rate <= 0 {
		return
	}
	if wait := c.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
