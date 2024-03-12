package service

import (
	"context"
	"strings"

	"github.com/araddon/dateparse"
	"github.com/kdwils/feedreader/pkg/parser"
	"github.com/kdwils/feedreader/storage"
)

type Service struct {
	store  storage.Storage
	parser parser.Parser
}

type CreateFeedRequest struct {
	Link string `json:"link"`
}

type CreateArticleRequest struct {
	storage.Article
}

func New(store storage.Storage, parser parser.Parser) Service {
	return Service{
		store:  store,
		parser: parser,
	}
}

func (s Service) CreateFeed(ctx context.Context, request CreateFeedRequest) (*storage.Feed, error) {
	parsedFeed, err := s.parser.ParseFromURI(ctx, request.Link)
	if err != nil {
		return nil, err
	}

	return s.store.CreateFeed(ctx, parsedFeed.Channel.Title, request.Link, parsedFeed.Channel.Link, parsedFeed.Channel.Description)
}

func (s Service) CreateArticle(ctx context.Context, request CreateArticleRequest) (*storage.Article, error) {
	publishedTime, err := dateparse.ParseAny(request.Published)
	if err != nil {
		return nil, err
	}

	return s.store.CreateArticle(ctx, request.Link, request.Title, request.Author, request.Description, publishedTime)
}

func (s Service) ListArticles(ctx context.Context, opts *storage.Options) (storage.ArticleList, error) {
	return s.store.ListArticles(ctx, opts)
}

func (s Service) ListFavoritedArticles(ctx context.Context, opts *storage.Options) (storage.ArticleList, error) {
	return s.store.ListFavoritedArticles(ctx, opts)
}

func (s Service) ListReadArticles(ctx context.Context, opts *storage.Options) (storage.ArticleList, error) {
	return s.store.ListReadArticles(ctx, opts)
}

func (s Service) ListUnreadArticles(ctx context.Context, opts *storage.Options) (storage.ArticleList, error) {
	return s.store.ListUnreadArticles(ctx, opts)
}

func (s Service) ListFeeds(ctx context.Context, opts *storage.Options) (storage.FeedList, error) {
	return s.store.ListFeeds(ctx, opts)
}

func (s Service) RefreshFeed(ctx context.Context, feed *storage.Feed) ([]*storage.Article, error) {
	feeds, err := s.parser.ParseFromURI(ctx, feed.RSSLink)
	if err != nil {
		return nil, err
	}

	articles, err := s.store.ListArticlesByFeed(ctx, feed.ID)
	if err != nil {
		return nil, err
	}

	newArticles := make([]parser.Item, 0)
	for _, fa := range feeds.Channel.Items {
		var contains bool
		for _, a := range articles {
			if strings.EqualFold(fa.Link, a.Link) {
				contains = true
			}
		}

		if !contains {
			newArticles = append(newArticles, fa)
		}
	}

	storedArticles := make([]*storage.Article, 0)
	for _, a := range newArticles {
		request := CreateArticleRequest{
			Article: storage.Article{
				Link:        a.Link,
				Title:       a.Title,
				Description: a.Description,
				Author:      a.Author,
				Published:   a.PubDate,
			},
		}

		new, err := s.CreateArticle(ctx, request)
		if err != nil {
			return nil, err
		}

		storedArticles = append(storedArticles, new)
	}

	return storedArticles, nil
}
