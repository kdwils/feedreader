package storage

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Storage interface {
	Connect() error
	Close() error

	CreateFeed(ctx context.Context, title, rssLink, siteLink, description string) (*Feed, error)
	ListFeeds(ctx context.Context, opts *Options) ([]*Feed, error)

	CreateArticle(ctx context.Context, link, title, author, description string, published time.Time) (*Article, error)
	ListArticles(ctx context.Context, opts *Options) ([]*Article, error)

	Now() time.Time
}

type Feed struct {
	ID          string `db:"id" json:"id"`
	Title       string `db:"title" json:"title"`
	SiteLink    string `db:"siteLink" json:"siteLink"`
	RSSLink     string `db:"rssLink" json:"rssLink"`
	Description string `db:"description" json:"description"`
	Timestamp   int64  `db:"timestamp" json:"-"`
}

type Article struct {
	ID            string `db:"id" json:"id"`
	FeedID        string `db:"feed" json:"feedID"`
	Link          string `db:"link" json:"link"`
	Title         string `db:"title" json:"title"`
	Description   string `db:"description" json:"description"`
	Published     string `db:"-" json:"publishedOn"`
	ReadDate      string `db:"readDate" json:"readDate"`
	Author        string `db:"author" json:"author"`
	PublishedUnix int64  `db:"published" json:"published"`
	Read          bool   `db:"read" json:"read"`
	Favorited     bool   `db:"favorited" json:"favorited"`
	Timestamp     int64  `db:"timestamp" json:"timestamp"`
}

type CreateFeedRequest struct {
	Link string `json:"link"`
}

type CreateArticleRequest struct {
	Article
}

type order string

const (
	Ascending  order = "ascending"
	Descending order = "descending"
)

type Options struct {
	Order  order
	Limit  int
	After  int
	Before int
}

func ParseOptions(req url.Values) *Options {
	opts := DefaultOptions()
	switch strings.ToLower(req.Get("order")) {
	case "descending":
		opts.Order = Descending
	case "ascending":
		opts.Order = Ascending
	}

	limit := req.Get("limit")
	if limit != "" {
		i, err := strconv.Atoi(limit)
		if err == nil {
			opts.Limit = i
		}
	}

	after := req.Get("after")
	if after != "" {
		i, err := strconv.Atoi(after)
		if err == nil {
			opts.After = i
		}
	}

	return opts
}

func DefaultOptions() *Options {
	return &Options{
		Order: Descending,
		Limit: 10,
		After: -1,
	}
}

func parseSiteLinkFromURI(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", err
	}

	parsed := url.URL{
		Host:   u.Host,
		Scheme: u.Scheme,
	}

	return parsed.String(), nil
}
