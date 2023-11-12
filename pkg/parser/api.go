package parser

import (
	"context"
	"io"
	"net/http"
)

// Parser describes how to parse an rss feed
//
//go:generate mockgen -destination=mocks/mock_parser.go -package=mocks github.com/kdwils/feedreader/pkg/parser Parser
type Parser interface {
	Parse(io.Reader) (*RSSFeed, error)
	ParseFromURI(ctx context.Context, uri string) (*RSSFeed, error)
}

// HTTP describes how to make an http request. This interface serves the purpose of providing a way to mock http requests.
//
//go:generate mockgen -destination=mocks/mock_http.go -package=mocks github.com/kdwils/feedreader/pkg/parser HTTP
type HTTP interface {
	Do(*http.Request) (*http.Response, error)
}

type RSSFeed struct {
	Channel Channel `xml:"channel"`
}

type Channel struct {
	Title         string `xml:"title"`
	Link          string `xml:"link"`
	Description   string `xml:"description"`
	Generator     string `xml:"generator"`
	LastBuildDate string `xml:"lastBuildDate"`
	Items         []Item `xml:"item"`
}

type Item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
	Description string `xml:"description"`
}
