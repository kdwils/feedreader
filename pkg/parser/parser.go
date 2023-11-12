package parser

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/url"
)

type FeedParser struct {
	http HTTP
}

func New(http HTTP) Parser {
	return FeedParser{
		http: http,
	}
}

type tokenBuffer struct {
	feed        *RSSFeed
	buffer      string
	data        bool
	openItemTag bool
}

func (tb *tokenBuffer) reset() {
	tb.buffer = ""
}

func (tb *tokenBuffer) ok() bool {
	return tb.buffer != ""
}

func (tb *tokenBuffer) itemsLen() int {
	if tb.feed == nil {
		return 0
	}

	return len(tb.feed.Channel.Items) - 1
}

func (fr FeedParser) Parse(reader io.Reader) (*RSSFeed, error) {
	decoder := xml.NewDecoder(reader)
	tb := &tokenBuffer{
		feed: new(RSSFeed),
	}

	for {
		token, err := decoder.Token()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return nil, err
			}
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			tb.handleStartElement(t)
		case xml.EndElement:
			tb.handleEndElement(t)
		case xml.CharData:
			tb.handleCharElement(t)
		}
	}

	return tb.feed, nil
}

func (tb *tokenBuffer) handleStartElement(e xml.StartElement) {
	if e.Name.Local == "item" {
		tb.openItemTag = true
		if tb.feed.Channel.Items == nil {
			tb.feed.Channel.Items = make([]Item, 0)
		}

		tb.feed.Channel.Items = append(tb.feed.Channel.Items, Item{})
	}

	tb.buffer = ""
	tb.data = true
}

func (tb *tokenBuffer) handleEndElement(e xml.EndElement) {
	// a closing element means we need to reset the buffer after its read because there is no more data to be parsed for that tag
	defer tb.reset()

	if !tb.ok() {
		return
	}

	switch e.Name.Local {
	case "generator":
		tb.feed.Channel.Generator = tb.buffer
	case "lastBuildDate":
		tb.feed.Channel.LastBuildDate = tb.buffer
	case "pubDate":
		tb.feed.Channel.Items[tb.itemsLen()].PubDate = tb.buffer
	case "guid":
		tb.feed.Channel.Items[tb.itemsLen()].GUID = tb.buffer
	case "title":
		if tb.openItemTag {
			tb.feed.Channel.Items[tb.itemsLen()].Title = tb.buffer
			return
		}

		tb.feed.Channel.Title = tb.buffer
	case "description":
		if tb.openItemTag {
			tb.feed.Channel.Items[tb.itemsLen()].Description = tb.buffer
			return
		}

		tb.feed.Channel.Description = tb.buffer
	case "link":
		u, err := url.Parse(tb.buffer)
		if err != nil {
			return
		}
		if tb.openItemTag {
			tb.feed.Channel.Items[tb.itemsLen()].Link = u.String()
			break
		}
		tb.feed.Channel.Link = u.String()
	case "item":
		tb.openItemTag = false
	}
}

func (tb *tokenBuffer) handleCharElement(e xml.CharData) {
	tb.buffer += string(e)
}

func (fr FeedParser) ParseFromURI(ctx context.Context, uri string) (*RSSFeed, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	resp, err := fr.http.Do(req)
	if err != nil {
		return nil, err
	}

	return fr.Parse(resp.Body)
}
