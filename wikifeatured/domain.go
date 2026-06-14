package wikifeatured

import (
	"context"
	"time"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

func init() { kit.Register(Domain{}) }

// Domain is the Wikipedia Featured Content driver.
type Domain struct{}

// Info describes the scheme, host, and identity.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "wikifeatured",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "wikifeatured",
			Short:  "Browse Wikipedia featured content: daily articles, most-read, news, and on-this-day events",
			Long: `wikifeatured reads Wikipedia's Featured Content REST API and prints clean
records to stdout. No API key required.

It covers four feeds: today's featured article (featured), most-read pages
(mostread), current news (news), and historical events (onthisday).`,
			Site: Host,
			Repo: "https://github.com/tamnd/wikifeatured-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "featured",
		Group:   "read",
		Single:  true,
		Summary: "Get today's featured article",
		URIType: "featured",
	}, getFeatured)

	kit.Handle(app, kit.OpMeta{
		Name:    "mostread",
		Group:   "read",
		List:    true,
		Summary: "List the most-read Wikipedia articles for a date",
		URIType: "article",
	}, getMostRead)

	kit.Handle(app, kit.OpMeta{
		Name:    "news",
		Group:   "read",
		List:    true,
		Summary: "List current news items from Wikipedia",
		URIType: "news",
	}, getNews)

	kit.Handle(app, kit.OpMeta{
		Name:    "onthisday",
		Group:   "read",
		List:    true,
		Summary: "List historical events for today's date",
		URIType: "event",
	}, getOnThisDay)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.HTTP.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- inputs ---

type featuredInput struct {
	Date   string  `kit:"flag" help:"date in YYYY/MM/DD format (default: today)"`
	Client *Client `kit:"inject"`
}

type mostReadInput struct {
	Date   string  `kit:"flag" help:"date in YYYY/MM/DD format (default: today)"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

type newsInput struct {
	Date   string  `kit:"flag" help:"date in YYYY/MM/DD format (default: today)"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

type onThisDayInput struct {
	Date   string  `kit:"flag" help:"date in MM/DD format (default: today)"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func getFeatured(ctx context.Context, in featuredInput, emit func(*Featured) error) error {
	date := in.Date
	if date == "" {
		date = time.Now().UTC().Format("2006/01/02")
	}
	f, err := in.Client.GetFeatured(ctx, date)
	if err != nil {
		return mapErr(err)
	}
	return emit(f)
}

func getMostRead(ctx context.Context, in mostReadInput, emit func(*Article) error) error {
	date := in.Date
	if date == "" {
		date = time.Now().UTC().Format("2006/01/02")
	}
	articles, err := in.Client.GetMostRead(ctx, date)
	if err != nil {
		return mapErr(err)
	}
	for i, a := range articles {
		if in.Limit > 0 && i >= in.Limit {
			break
		}
		if err := emit(a); err != nil {
			return err
		}
	}
	return nil
}

func getNews(ctx context.Context, in newsInput, emit func(*NewsItem) error) error {
	date := in.Date
	if date == "" {
		date = time.Now().UTC().Format("2006/01/02")
	}
	items, err := in.Client.GetNews(ctx, date)
	if err != nil {
		return mapErr(err)
	}
	for i, item := range items {
		if in.Limit > 0 && i >= in.Limit {
			break
		}
		if err := emit(item); err != nil {
			return err
		}
	}
	return nil
}

func getOnThisDay(ctx context.Context, in onThisDayInput, emit func(*Event) error) error {
	date := in.Date
	if date == "" {
		date = time.Now().UTC().Format("01/02")
	}
	events, err := in.Client.GetOnThisDay(ctx, date)
	if err != nil {
		return mapErr(err)
	}
	for i, e := range events {
		if in.Limit > 0 && i >= in.Limit {
			break
		}
		if err := emit(e); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver ---

// Classify satisfies kit.Domain. This domain does not classify arbitrary URLs
// into addressable resource URIs, so we return a usage error for any input.
func (Domain) Classify(input string) (uriType, id string, err error) {
	return "", "", errs.Usage("wikifeatured does not resolve arbitrary URLs; use featured/mostread/news/onthisday commands")
}

// Locate satisfies kit.Domain.
func (Domain) Locate(uriType, id string) (string, error) {
	return "", errs.Usage("wikifeatured has no resource type %q", uriType)
}

// mapErr passes errors through unchanged; add specific error kinds here as needed.
func mapErr(err error) error {
	return err
}
