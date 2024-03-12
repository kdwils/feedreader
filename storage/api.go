package storage

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Storage interface {
	Connect() error
	Close() error

	CreateFeed(ctx context.Context, title, rssLink, siteLink, description string) (*Feed, error)
	ListFeeds(ctx context.Context, opts *Options) (FeedList, error)

	CreateArticle(ctx context.Context, link, title, author, description string, published time.Time) (*Article, error)
	ListArticles(ctx context.Context, opts *Options) (ArticleList, error)
	ListArticlesByFeed(ctx context.Context, feed string) ([]*Article, error)
	ListUnreadArticles(ctx context.Context, opts *Options) (ArticleList, error)
	ListReadArticles(ctx context.Context, opts *Options) (ArticleList, error)

	Now() time.Time
}

type Cursor struct {
	Next    string `json:"next"`
	Prev    string `json:"prev"`
	HasNext bool   `json:"hasNext"`
	HasPrev bool   `json:"hasPrev"`
}

type CursorItem interface {
	GetPaginationField() string
}

type FeedList struct {
	Cursor `json:"cursor"`
	Feeds  []*Feed `json:"feeds"`
}

type ArticleList struct {
	Cursor   `json:"cursor"`
	Articles []*Article `json:"articles"`
}

func getPagination[T CursorItem](next, prev []T, limit int, maximumPaginatedValue string) ([]T, Cursor) {
	var hasNext bool
	var nextCursor string
	var prevCursor string
	var hasPrev bool
	maxLimit := limit + 1

	if len(next) == 0 {
		hasNext = false
		nextCursor = ""
	}
	if len(next) == maxLimit {
		hasNext = true
		// return 2nd to last, it is assumed queries are doing limit +1 and therefore don't care about last item
		nextCursor = next[len(next)-1-1].GetPaginationField()
	}

	// at least 1 item up to exact match on limit means there is are no further pages
	if len(next) > 1 && len(next) <= limit-1 {
		hasNext = false
		nextCursor = next[len(next)-1].GetPaginationField()
	}

	if len(prev) == 0 {
		hasPrev = false
		prevCursor = ""
	}

	if len(prev) == limit {
		hasPrev = true
		prevCursor = prev[1].GetPaginationField()
	}

	if len(prev) > 1 && len(prev) <= limit-1 {
		hasPrev = true
		prevCursor = maximumPaginatedValue
	}

	if len(next) == maxLimit {
		next = next[:limit]
	}

	return next, Cursor{
		HasNext: hasNext,
		Next:    nextCursor,
		HasPrev: hasPrev,
		Prev:    prevCursor,
	}
}

type Feed struct {
	ID          string `db:"id" json:"id"`
	Title       string `db:"title" json:"title"`
	SiteLink    string `db:"siteLink" json:"siteLink"`
	RSSLink     string `db:"rssLink" json:"rssLink"`
	Description string `db:"description" json:"description"`
	Timestamp   int64  `db:"timestamp" json:"-"`
}

func (f *Feed) GetPaginationField() string {
	return f.ID
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

func (a *Article) GetPaginationField() string {
	return fmt.Sprintf("%d", a.PublishedUnix)
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

func (o order) string() string {
	switch o {
	case Ascending:
		return "ASC"
	case Descending:
		return "DESC"
	default:
		return "DESC"
	}
}

func (o order) opposite() string {
	switch o {
	case Ascending:
		return "DESC"
	case Descending:
		return "ASC"
	default:
		return "ASC"
	}
}

type Options struct {
	Cursor string
	Order  order
	Limit  int
}

func ParseOptions(req url.Values) *Options {
	opts := DefaultOptions()
	limit := req.Get("limit")
	if limit != "" {
		i, err := strconv.Atoi(limit)
		if err == nil {
			opts.Limit = i
		}
	}
	opts.Cursor = req.Get("cursor")

	if order := req.Get("order"); order != "" {
		switch strings.ToLower(order) {
		case string(Descending):
			opts.Order = Descending
		case string(Ascending):
			opts.Order = Ascending
		}
	}

	return opts
}

func DefaultOptions() *Options {
	return &Options{
		Limit:  10,
		Cursor: "",
		Order:  Descending,
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
