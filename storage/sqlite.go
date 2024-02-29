package storage

import (
	"context"
	"database/sql"
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

func (s *SQLite) ListFeeds(ctx context.Context, opts *Options) ([]*Feed, error) {
	if s.db == nil {
		return nil, ErrNilDB
	}

	if opts == nil {
		opts = DefaultOptions()
	}

	query := "SELECT * FROM feeds where id > ? LIMIT ?"
	stmt, err := s.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, nil
	}
	defer stmt.Close()

	feeds := make([]*Feed, 0)

	rows, err := stmt.QueryContext(ctx, opts.After, opts.Limit)
	if err != nil {
		if errors.Is(sql.ErrNoRows, err) {
			return feeds, nil
		}
	}

	for rows.Next() {
		var f Feed
		err = rows.Scan(&f.ID, &f.Title, &f.RSSLink, &f.SiteLink, &f.Description, &f.Timestamp)
		if err != nil {
			return nil, err
		}

		feeds = append(feeds, &f)
	}

	return feeds, nil
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

	result, err := stmt.ExecContext(ctx, feed.ID, article.Link, article.Title, article.Author, article.Description, article.Published, article.ReadDate, article.Read, article.Favorited, article.Timestamp)
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

func (s *SQLite) ListArticles(ctx context.Context, opts *Options) ([]*Article, error) {
	if s.db == nil {
		return nil, ErrNilDB
	}

	if opts == nil {
		opts = DefaultOptions()
	}

	query := "SELECT * FROM articles ORDER BY published DESC LIMIT ?"
	stmt, err := s.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	articles := make([]*Article, 0)
	rows, err := stmt.QueryContext(ctx, opts.Limit)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var a Article
		err = rows.Scan(&a.ID, &a.FeedID, &a.Link, &a.Title, &a.Author, &a.Description, &a.Published, &a.Read, &a.ReadDate, &a.Favorited, &a.Timestamp)
		if err != nil {
			return nil, err
		}
		articles = append(articles, &a)
	}

	return articles, nil
}
