package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type SQLite struct {
	db       *sqlx.DB
	filePath string
}

const (
	maxPublishedDate = "9999999999"
	maxFeedID        = "9999999999"
)

func NewSQLiteStorage(filePath string) Storage {
	return &SQLite{
		filePath: filePath,
	}
}

func (s *SQLite) Now() time.Time {
	return time.Now()
}

func (s *SQLite) Connect() error {
	db, err := sqlx.Open("sqlite3", s.filePath)
	if err != nil {
		return err
	}

	s.db = db

	init := `
	CREATE TABLE IF NOT EXISTS feeds (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		rssLink TEXT NOT NULL UNIQUE,
		siteLink TEXT NOT NULL UNIQUE,
		description TEXT NOT NULL,
		timestamp INT NOT NULL
	);
	
	CREATE TABLE IF NOT EXISTS articles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		feed INTEGER NOT NULL,
		title TEXT NOT NULL,
		author TEXT NOT NULL,
		description TEXT NOT NULL,
		link TEXT NOT NULL UNIQUE,
		published TEXT NOT NULL,
		read BOOLEAN NOT NULL,
		read_date TEXT NOT NULL,
		favorited BOOLEAN NOT NULL,
		timestamp INT NOT NULL,
		FOREIGN KEY(feed) REFERENCES feeds(id)
	);`

	_, err = db.Exec(init)
	return err
}

func (s *SQLite) Close() error {
	return s.db.Close()
}

func (s *SQLite) CreateFeed(ctx context.Context, title, rssLink, siteLink, description string) (*Feed, error) {
	if s.db == nil {
		return nil, ErrNilDB
	}

	if rssLink == "" {
		return nil, errors.New("feed link is empty")
	}

	if title == "" {
		return nil, errors.New("feed title is empty")
	}

	siteLink, err := parseSiteLinkFromURI(rssLink)
	if err != nil {
		return nil, err
	}

	query := "INSERT INTO feeds (title, siteLink, rssLink, description, timestamp) VALUES (?, ?, ?, ?, ?)"

	stmt, err := s.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	f := &Feed{
		Title:       title,
		SiteLink:    siteLink,
		RSSLink:     rssLink,
		Description: description,
		Timestamp:   s.Now().UTC().Unix(),
	}

	result, err := stmt.ExecContext(ctx, f.Title, f.SiteLink, f.RSSLink, f.Description, f.Timestamp)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return f, err
	}

	f.ID = fmt.Sprintf("%d", id)
	return f, nil
}

func (s *SQLite) getFeedByLink(ctx context.Context, link string) (Feed, error) {
	query := "SELECT * FROM feeds WHERE siteLink = ?"
	stmt, err := s.db.PrepareContext(ctx, query)

	var f Feed
	if err != nil {
		return f, err
	}

	row := stmt.QueryRow(link)
	if row.Err() != nil {
		return f, row.Err()
	}

	err = row.Scan(&f.ID, &f.Title, &f.SiteLink, &f.RSSLink, &f.Description, &f.Timestamp)
	return f, err
}

func (s *SQLite) ListFeeds(ctx context.Context, opts *Options) (FeedList, error) {
	if s.db == nil {
		return FeedList{}, ErrNilDB
	}

	if opts == nil {
		opts = DefaultOptions()
	}

	limit := opts.Limit + 1

	nextQuery := fmt.Sprintf("SELECT * FROM feeds WHERE id < ? ORDER BY id %s LIMIT %d", Descending.string(), limit)
	prevQuery := fmt.Sprintf("SELECT * FROM ( SELECT * FROM feeds WHERE id > ? ORDER BY id %s LIMIT %d ) AS data ORDER BY id %s", Ascending.string(), limit, Descending.string())

	return s.doFeedQueries(ctx, nextQuery, prevQuery, opts.Cursor, opts.Limit)
}

func (s *SQLite) doFeedQueries(ctx context.Context, nextQuery, prevQuery, cursor string, limit int) (FeedList, error) {
	feedList := FeedList{
		Feeds: make([]*Feed, 0),
	}

	nextStmt, err := s.db.PrepareContext(ctx, nextQuery)
	if err != nil {
		return feedList, err
	}

	nextPagination := cursor
	if nextPagination == "" {
		nextPagination = maxFeedID
	}
	next, err := nextStmt.QueryContext(ctx, nextPagination)
	if err != nil {
		return feedList, err
	}

	nextFeeds := make([]*Feed, 0)
	for next.Next() {
		var f Feed
		err = next.Scan(&f.ID, &f.Title, &f.RSSLink, &f.SiteLink, &f.Description, &f.Timestamp)
		if err != nil {
			return feedList, err
		}

		nextFeeds = append(nextFeeds, &f)
	}

	prevStmt, err := s.db.PrepareContext(ctx, prevQuery)
	if err != nil {
		return feedList, err
	}

	prev, err := prevStmt.QueryContext(ctx, cursor)
	if err != nil {
		return feedList, err
	}

	prevFeeds := make([]*Feed, 0)
	for prev.Next() {
		var f Feed
		err = next.Scan(&f.ID, &f.Title, &f.RSSLink, &f.SiteLink, &f.Description, &f.Timestamp)
		if err != nil {
			return feedList, err
		}

		prevFeeds = append(prevFeeds, &f)
	}

	nextArticles, nextCursor := getPagination(nextFeeds, prevFeeds, limit, maxFeedID)
	feedList.Feeds = nextArticles
	feedList.Cursor = nextCursor
	return feedList, nil
}

func (s *SQLite) CreateArticle(ctx context.Context, link, title, author, description string, published time.Time) (*Article, error) {
	if s.db == nil {
		return nil, ErrNilDB
	}

	if link == "" {
		return nil, errors.New("article link is empty")
	}

	if title == "" {
		return nil, errors.New("article title is empty")
	}

	if author == "" {
		return nil, errors.New("article author is empty")
	}

	feedLink, err := parseSiteLinkFromURI(link)
	if err != nil {
		return nil, err
	}

	feed, err := s.getFeedByLink(ctx, feedLink)
	if err != nil {
		return nil, err
	}

	query := "INSERT INTO articles (feed, link, title, author, description, published, read_date, read, favorited, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	stmt, err := s.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	article := &Article{
		Link:          link,
		FeedID:        feed.ID,
		Title:         title,
		Description:   description,
		Author:        author,
		PublishedUnix: published.UTC().Unix(),
		ReadDate:      "",
		Favorited:     false,
		Read:          false,
		Timestamp:     s.Now().UTC().Unix(),
	}

	result, err := stmt.ExecContext(ctx, feed.ID, article.Link, article.Title, article.Author, article.Description, article.PublishedUnix, article.ReadDate, article.Read, article.Favorited, article.Timestamp)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	article.ID = fmt.Sprintf("%d", id)
	return article, nil
}

func (s *SQLite) ListReadArticles(ctx context.Context, opts *Options) (ArticleList, error) {
	var articleList ArticleList

	if s.db == nil {
		return articleList, ErrNilDB
	}

	if opts == nil {
		opts = DefaultOptions()
	}

	limit := opts.Limit + 1

	nextQuery := fmt.Sprintf("SELECT * FROM articles WHERE read = true AND published < ? ORDER BY published %s LIMIT %d", Descending.string(), limit)

	prevQuery := fmt.Sprintf("SELECT * FROM ( SELECT * FROM articles WHERE read = true AND published > ? ORDER BY published %s LIMIT %d ) AS data ORDER BY published %s", Ascending.string(), limit, Descending.string())

	articleList, err := s.doArticleQueries(ctx, nextQuery, prevQuery, opts.Cursor, opts.Limit)
	return articleList, err
}

func (s *SQLite) ListUnreadArticles(ctx context.Context, opts *Options) (ArticleList, error) {
	var articleList ArticleList

	if s.db == nil {
		return articleList, ErrNilDB
	}

	if opts == nil {
		opts = DefaultOptions()
	}

	limit := opts.Limit + 1

	nextQuery := fmt.Sprintf("SELECT * FROM articles WHERE read = false AND published < ? ORDER BY published %s LIMIT %d", Descending.string(), limit)

	prevQuery := fmt.Sprintf("SELECT * FROM ( SELECT * FROM articles WHERE read = false AND published > ? ORDER BY published %s LIMIT %d ) AS data ORDER BY published %s", Ascending.string(), limit, Descending.string())

	articleList, err := s.doArticleQueries(ctx, nextQuery, prevQuery, opts.Cursor, opts.Limit)
	return articleList, err
}

func (s *SQLite) ListFavoritedArticles(ctx context.Context, opts *Options) (ArticleList, error) {
	var articleList ArticleList

	if s.db == nil {
		return articleList, ErrNilDB
	}

	if opts == nil {
		opts = DefaultOptions()
	}

	limit := opts.Limit + 1

	nextQuery := fmt.Sprintf("SELECT * FROM articles WHERE favorited = true AND published < ? ORDER BY published %s LIMIT %d", Descending.string(), limit)

	prevQuery := fmt.Sprintf("SELECT * FROM ( SELECT * FROM articles WHERE favorited = true AND published > ? ORDER BY published %s LIMIT %d ) AS data ORDER BY published %s", Ascending.string(), limit, Descending.string())

	articleList, err := s.doArticleQueries(ctx, nextQuery, prevQuery, opts.Cursor, opts.Limit)
	return articleList, err
}

func (s *SQLite) doArticleQueries(ctx context.Context, nextQuery, prevQuery, cursor string, limit int) (ArticleList, error) {
	articleList := ArticleList{
		Articles: make([]*Article, 0),
	}

	nextStmt, err := s.db.PrepareContext(ctx, nextQuery)
	if err != nil {
		return articleList, err
	}

	nextPagination := cursor
	if nextPagination == "" {
		nextPagination = maxPublishedDate
	}
	next, err := nextStmt.QueryContext(ctx, nextPagination)
	if err != nil {
		return articleList, err
	}

	nextArticles := make([]*Article, 0)
	for next.Next() {
		var a Article
		err = next.Scan(&a.ID, &a.FeedID, &a.Title, &a.Author, &a.Description, &a.Link, &a.PublishedUnix, &a.Read, &a.ReadDate, &a.Favorited, &a.Timestamp)
		if err != nil {
			return articleList, err
		}
		a.Published = time.Unix(a.PublishedUnix, 0).UTC().Format("Mon, 02 Jan 2006")
		nextArticles = append(nextArticles, &a)
	}

	prevStmt, err := s.db.PrepareContext(ctx, prevQuery)
	if err != nil {
		return articleList, err
	}

	prev, err := prevStmt.QueryContext(ctx, cursor)
	if err != nil {
		return articleList, err
	}

	prevArticles := make([]*Article, 0)
	for prev.Next() {
		var a Article
		err = prev.Scan(&a.ID, &a.FeedID, &a.Title, &a.Author, &a.Description, &a.Link, &a.PublishedUnix, &a.Read, &a.ReadDate, &a.Favorited, &a.Timestamp)
		if err != nil {
			return articleList, err
		}
		a.Published = time.Unix(a.PublishedUnix, 0).UTC().Format("Mon, 02 Jan 2006")
		prevArticles = append(prevArticles, &a)
	}

	nextArticles, nextCursor := getPagination(nextArticles, prevArticles, limit, maxPublishedDate)
	articleList.Articles = nextArticles
	articleList.Cursor = nextCursor
	return articleList, nil
}

func (s *SQLite) ListArticles(ctx context.Context, opts *Options) (ArticleList, error) {
	articleList := ArticleList{
		Articles: make([]*Article, 0),
	}

	if s.db == nil {
		return articleList, ErrNilDB
	}

	if opts == nil {
		opts = DefaultOptions()
	}

	limit := opts.Limit + 1

	nextQuery := fmt.Sprintf("SELECT * FROM articles WHERE read = false AND published < ? ORDER BY published %s LIMIT %d", Descending.string(), limit)
	nextStmt, err := s.db.PrepareContext(ctx, nextQuery)
	if err != nil {
		return articleList, err
	}

	nextPagination := opts.Cursor
	if nextPagination == "" {
		nextPagination = maxPublishedDate
	}
	next, err := nextStmt.QueryContext(ctx, nextPagination)
	if err != nil {
		return articleList, err
	}

	nextArticles := make([]*Article, 0)
	for next.Next() {
		var a Article
		err = next.Scan(&a.ID, &a.FeedID, &a.Title, &a.Author, &a.Description, &a.Link, &a.PublishedUnix, &a.Read, &a.ReadDate, &a.Favorited, &a.Timestamp)
		if err != nil {
			return articleList, err
		}
		a.Published = time.Unix(a.PublishedUnix, 0).UTC().Format("Mon, 02 Jan 2006")
		nextArticles = append(nextArticles, &a)
	}

	prevQuery := fmt.Sprintf("SELECT * FROM ( SELECT * FROM articles WHERE read = false AND published > ? ORDER BY published %s LIMIT %d ) AS data ORDER BY published %s", Ascending.string(), limit, Descending.string())
	prevStmt, err := s.db.PrepareContext(ctx, prevQuery)
	if err != nil {
		return articleList, err
	}

	prev, err := prevStmt.QueryContext(ctx, opts.Cursor)
	if err != nil {
		return articleList, err
	}

	prevArticles := make([]*Article, 0)
	for prev.Next() {
		var a Article
		err = prev.Scan(&a.ID, &a.FeedID, &a.Title, &a.Author, &a.Description, &a.Link, &a.PublishedUnix, &a.Read, &a.ReadDate, &a.Favorited, &a.Timestamp)
		if err != nil {
			return articleList, err
		}
		a.Published = time.Unix(a.PublishedUnix, 0).UTC().Format("Mon, 02 Jan 2006")
		prevArticles = append(prevArticles, &a)
	}

	nextArticles, cursor := getPagination(nextArticles, prevArticles, opts.Limit, maxPublishedDate)
	articleList.Articles = nextArticles
	articleList.Cursor = cursor
	return articleList, nil
}

// func (s *SQLite) ListArticles(ctx context.Context, opts *Options) (ArticleList, error) {
// 	articleList := ArticleList{
// 		Articles: make([]*Article, 0),
// 	}

// 	if s.db == nil {
// 		return articleList, ErrNilDB
// 	}

// 	if opts == nil {
// 		opts = DefaultOptions()
// 	}

// 	query := "SELECT * FROM articles"
// 	if opts.After != "" || opts.Before != "" {
// 		query = "SELECT * FROM articles WHERE"
// 	}

// 	var args []interface{}

// 	if opts.After != "" {
// 		query += " published > ?"
// 		args = append(args, opts.After)
// 	}

// 	if opts.Before != "" {
// 		if opts.After != "" {
// 			query += " AND"
// 		}
// 		query += " published < ?"
// 		args = append(args, opts.Before)
// 	}

// 	// Adjust the limit based on whether it's the first page or not
// 	var limit int
// 	if opts.After == "" && opts.Before == "" {
// 		limit = opts.Limit + 1
// 	} else {
// 		limit = opts.Limit
// 	}

// 	query += " ORDER BY published " + opts.Order.string() + ", id " + opts.Order.opposite() + " LIMIT ?"
// 	args = append(args, limit)

// 	stmt, err := s.db.PrepareContext(ctx, query)
// 	if err != nil {
// 		return articleList, err
// 	}
// 	defer stmt.Close()

// 	rows, err := stmt.QueryContext(ctx, args...)
// 	if err != nil {
// 		return articleList, err
// 	}
// 	defer rows.Close()

// 	for rows.Next() {
// 		var a Article
// 		err = rows.Scan(&a.ID, &a.FeedID, &a.Title, &a.Author, &a.Description, &a.Link, &a.PublishedUnix, &a.Read, &a.ReadDate, &a.Favorited, &a.Timestamp)
// 		if err != nil {
// 			return articleList, err
// 		}
// 		a.Published = time.Unix(a.PublishedUnix, 0).UTC().Format("Mon, 02 Jan 2006")
// 		articleList.Articles = append(articleList.Articles, &a)
// 	}

// 	// Set next and previous cursors
// 	var cursor Cursor
// 	hasNext := len(articleList.Articles) > opts.Limit
// 	if hasNext {
// 		cursor.HasNext = true
// 		cursor.HasPrev = opts.Before != ""
// 		if len(articleList.Articles) == limit {
// 			// If there are more items than the limit, set next cursor to the published field of the second to last article fetched
// 			cursor.Next = articleList.Articles[len(articleList.Articles)-2].GetPaginationField()
// 		}
// 		if opts.Before != "" {
// 			cursor.Prev = articleList.Articles[0].GetPaginationField()
// 		}
// 	}
// 	articleList.Cursor = cursor

// 	// Trim the list if necessary
// 	if hasNext {
// 		articleList.Articles = articleList.Articles[:opts.Limit]
// 	}

// 	return articleList, nil
// }

func (s *SQLite) ListArticlesByFeed(ctx context.Context, feedID string) ([]*Article, error) {
	if s.db == nil {
		return nil, ErrNilDB
	}

	query := "SELECT * FROM articles WHERE feed = ?"
	stmt, err := s.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}

	rows, err := stmt.QueryContext(ctx, feedID)
	if err != nil {
		return nil, err
	}

	articles := make([]*Article, 0)
	for rows.Next() {
		var a Article
		err = rows.Scan(&a.ID, &a.FeedID, &a.Title, &a.Author, &a.Description, &a.Link, &a.Published, &a.Read, &a.ReadDate, &a.Favorited, &a.Timestamp)
		if err != nil {
			return nil, err
		}
		articles = append(articles, &a)
	}

	return articles, nil
}
