package wikifeatured

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// featuredPayload builds a minimal /feed/featured/{date} response.
func featuredPayload(t *testing.T) []byte {
	t.Helper()
	payload := map[string]any{
		"tfa": map[string]any{
			"title":   "Go_(programming_language)",
			"extract": "Go is a statically typed, compiled programming language.",
		},
		"mostread": map[string]any{
			"articles": []any{
				map[string]any{
					"rank":    1,
					"title":   "Python_(programming_language)",
					"views":   123456,
					"extract": "Python is a high-level, general-purpose programming language.",
				},
				map[string]any{
					"rank":    2,
					"title":   "Rust_(programming_language)",
					"views":   98765,
					"extract": "Rust is a multi-paradigm, general-purpose programming language.",
				},
			},
		},
		"news": []any{
			map[string]any{
				"story": "<b>Breaking</b>: <a href=\"/wiki/Go\">Go</a> 1.22 released.",
				"links": []any{
					map[string]any{"title": "Go_(programming_language)"},
					map[string]any{"title": "Google"},
				},
			},
			map[string]any{
				"story": "Another news item.",
				"links": []any{},
			},
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// onThisDayPayload builds a minimal /feed/onthisday/events/{date} response.
func onThisDayPayload(t *testing.T) []byte {
	t.Helper()
	payload := map[string]any{
		"events": []any{
			map[string]any{"year": 1944, "text": "D-Day: Allied forces land in Normandy."},
			map[string]any{"year": 2017, "text": "The Grenfell Tower fire broke out in London."},
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func newTestClient(srv *httptest.Server) *Client {
	c := NewClient()
	c.Rate = 0
	c.BaseURL = srv.URL
	return c
}

func TestGetFeatured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(featuredPayload(t))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	f, err := c.GetFeatured(context.Background(), "2026/06/14")
	if err != nil {
		t.Fatal(err)
	}
	if f.Title != "Go_(programming_language)" {
		t.Errorf("Title = %q, want Go_(programming_language)", f.Title)
	}
	if f.Date != "2026/06/14" {
		t.Errorf("Date = %q, want 2026/06/14", f.Date)
	}
	if f.Extract == "" {
		t.Error("Extract is empty")
	}
}

func TestGetMostRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(featuredPayload(t))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	articles, err := c.GetMostRead(context.Background(), "2026/06/14")
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 2 {
		t.Fatalf("len(articles) = %d, want 2", len(articles))
	}
	if articles[0].Rank != 1 {
		t.Errorf("articles[0].Rank = %d, want 1", articles[0].Rank)
	}
	if articles[0].Title != "Python_(programming_language)" {
		t.Errorf("articles[0].Title = %q", articles[0].Title)
	}
	if articles[0].Views != 123456 {
		t.Errorf("articles[0].Views = %d, want 123456", articles[0].Views)
	}
}

func TestGetNews(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(featuredPayload(t))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	items, err := c.GetNews(context.Background(), "2026/06/14")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	// HTML tags must be stripped.
	story := items[0].Story
	if story == "" {
		t.Error("Story is empty")
	}
	for _, ch := range []string{"<b>", "</b>", "<a ", "</a>"} {
		if contains(story, ch) {
			t.Errorf("Story still contains HTML: %q in %q", ch, story)
		}
	}
	if items[0].LinkCount != 2 {
		t.Errorf("LinkCount = %d, want 2", items[0].LinkCount)
	}
}

func TestGetOnThisDay(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(onThisDayPayload(t))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	events, err := c.GetOnThisDay(context.Background(), "06/14")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Year != 1944 {
		t.Errorf("events[0].Year = %d, want 1944", events[0].Year)
	}
	if events[0].Text == "" {
		t.Error("events[0].Text is empty")
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(onThisDayPayload(t))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.GetOnThisDay(context.Background(), "06/14")
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsRune(s, sub))
}

func containsRune(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
