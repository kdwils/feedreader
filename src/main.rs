mod db;

use anyhow::Result;
use askama::Template;
use base64::{engine::general_purpose, Engine as _};
use chrono::{DateTime, SecondsFormat, Utc};
use core::panic;
use feed_rs::parser;
use futures::stream::StreamExt;
use futures::{future, stream};
use rweb::*;
use serde::{Deserialize, Serialize};
use std::{env, str::FromStr, vec};
use tokio::signal::unix::{signal, SignalKind};
use tokio::time;
use tokio_stream::wrappers::{IntervalStream, SignalStream};

const DEFAULT_REFRESH_SECONDS: u64 = 3 * 60;

#[derive(Debug)]
struct AppError(anyhow::Error);
impl rweb::reject::Reject for AppError {}

fn reject_anyhow(err: anyhow::Error) -> Rejection {
    warp::reject::custom(AppError(err))
}

#[derive(Deserialize, Serialize)]
struct Healthz {
    up: bool,
}

#[derive(Template)]
#[template(path = "feeds.html")]
struct FeedsTemplate {
    cursor: db::Cursor,
    feeds: Vec<Feed>,
}

#[derive(Template)]
#[template(path = "feed_list.html")]
struct FeedListTemplate {
    cursor: db::Cursor,
    feeds: Vec<Feed>,
}

#[derive(Template)]
#[template(path = "add_feed.html")]
struct AddFeedTemplate {}

#[derive(Template)]
#[template(path = "article_list.html")]
struct ArticleListTemplate {
    cursor: db::Cursor,
    articles: Vec<Article>,
}

#[derive(Template, Default)]
#[template(path = "articles.html")]
struct ArticleBaseTemplate {
    article_filter: String,
    title: String,
    cursor: db::Cursor,
    articles: Vec<Article>,
}

#[derive(Debug)]
struct BadActionError();
impl rweb::reject::Reject for BadActionError {}

#[derive(Deserialize, Serialize, Clone, Debug)]
pub struct Feed {
    id: String,
    name: String,
    site_url: String,
    feed_url: String,
    date_added: String,
    last_updated: String,
}

impl Feed {
    pub fn new(name: String, site_url: String, feed_url: String) -> Self {
        Feed {
            id: general_purpose::URL_SAFE.encode(feed_url.clone()),
            name,
            site_url,
            feed_url,
            date_added: Utc::now()
                .to_rfc3339_opts(SecondsFormat::Millis, true)
                .to_string(),
            last_updated: "-1".to_string(),
        }
    }
}

impl From<&tokio_postgres::Row> for Feed {
    fn from(row: &tokio_postgres::Row) -> Self {
        Feed {
            id: row.get(0),
            name: row.get(1),
            site_url: row.get(2),
            feed_url: row.get(3),
            date_added: row.get(4),
            last_updated: row.get(5),
        }
    }
}

#[derive(Serialize, Deserialize)]
struct AddFeed {
    feed_name: String,
    site_url: String,
    feed_url: String,
}

#[derive(Deserialize, Serialize, Clone, Debug)]
pub struct Article {
    id: String,
    feed: String,
    title: String,
    link: String,
    author: String,
    published: String,
    read: bool,
    favorited: bool,
    read_date: String,
}

impl Article {
    pub fn new(
        title: String,
        link: String,
        author: String,
        published: String,
        read: bool,
        favorited: bool,
    ) -> Self {
        Article {
            id: general_purpose::URL_SAFE_NO_PAD.encode(link.clone()),
            feed: "".to_string(),
            title,
            link,
            author,
            published: match DateTime::parse_from_rfc2822(published.as_str()) {
                Ok(dt) => dt.to_rfc3339_opts(SecondsFormat::Secs, true).to_string(),
                Err(_) => published,
            },
            read,
            favorited,
            read_date: "-1".to_string(),
        }
    }

    pub fn rfc3339_timestamp() -> String {
        Utc::now()
            .to_rfc3339_opts(SecondsFormat::Millis, true)
            .to_string()
    }

    pub fn rfc3339_timestamp_to_human(timestamp: String) -> String {
        match DateTime::parse_from_rfc3339(timestamp.as_str()) {
            Ok(dt) => dt.format("%m/%d/%Y").to_string(),
            Err(_) => timestamp,
        }
    }
}

impl From<&tokio_postgres::Row> for Article {
    fn from(row: &tokio_postgres::Row) -> Self {
        Article {
            id: row.get(0),
            feed: row.get(1),
            title: row.get(2),
            link: row.get(3),
            author: row.get(4),
            published: Article::rfc3339_timestamp_to_human(row.get(5)),
            read: row.get(6),
            favorited: row.get(7),
            read_date: Article::rfc3339_timestamp_to_human(row.get(8)),
        }
    }
}

impl From<&feed_rs::model::Entry> for Article {
    fn from(value: &feed_rs::model::Entry) -> Self {
        let title = match value.title.clone() {
            Some(text) => text.content.to_string(),
            None => "".to_string(),
        };

        let link = value
            .links
            .iter()
            .take(1)
            .map(|l| l.href.to_string())
            .next()
            .unwrap_or_else(|| "".to_string());

        let author = value
            .authors
            .iter()
            .take(1)
            .map(|p| p.name.to_string())
            .next()
            .unwrap_or_else(|| "".to_string());

        let timestamp = if let Some(_published) = value.published {
            value.published
        } else if let Some(_updated) = value.updated {
            value.updated
        } else {
            None
        };

        let published = match timestamp {
            Some(ts) => ts.to_rfc3339_opts(SecondsFormat::Millis, true),
            None => "".to_string(),
        };

        Article::new(title, link, author, published, false, false)
    }
}

#[tokio::main]
async fn main() {
    let db_username = env::var("POSTGRES_USERNAME").unwrap();
    let db_password = env::var("POSTGRES_PASSWORD").unwrap();
    let db_host = env::var("POSTGRES_HOST").unwrap_or("0.0.0.0".to_string());
    let db_port = env::var("POSTGRES_PORT")
        .unwrap_or("5432".to_string())
        .parse()
        .unwrap();

    let store = db::connection(
        db_username.as_str(),
        db_password.as_str(),
        db_host.as_str(),
        db_port,
    )
    .await
    .unwrap();

    match store.init().await {
        Ok(_) => (),
        Err(e) => panic!("could not init db: {}", e.to_string()),
    }

    let cors = warp::cors()
        .allow_any_origin()
        .allow_headers(vec![
            "Authorization",
            "Content-Type",
            "User-Agent",
            "Sec-Fetch-Mode",
            "Referer",
            "Origin",
            "Access-Control-Request-Method",
            "Access-Control-Request-Headers",
            "article_filter",
            "pagination",
        ])
        .allow_methods(vec!["GET", "HEAD", "POST", "DELETE"]);

    let routes = healthz()
        .or(index(store.clone()))
        .or(favorites(store.clone()))
        .or(history(store.clone()))
        .or(get_articles(store.clone()))
        .or(mark_article_read(store.clone()))
        .or(mark_article_favorite(store.clone()))
        .or(create_feed(store.clone()))
        .or(feeds(store.clone()))
        .or(delete_feed(store.clone()))
        .or(add_feed())
        .or(refresh_feed(store.clone()))
        .with(cors);

    let refresh_seconds = match env::var("FEED_REFRESH_SECONDS") {
        Ok(s) => s.parse().unwrap_or(DEFAULT_REFRESH_SECONDS),
        Err(_) => DEFAULT_REFRESH_SECONDS,
    };

    let mut exit = stream::select_all(vec![
        SignalStream::new(signal(SignalKind::interrupt()).unwrap()),
        SignalStream::new(signal(SignalKind::terminate()).unwrap()),
        SignalStream::new(signal(SignalKind::quit()).unwrap()),
    ]);

    let refresh_store = store.clone();
    let refresh_stream =
        IntervalStream::new(time::interval(time::Duration::from_secs(refresh_seconds)))
            .take_until(exit.next())
            .for_each(|_| async {
                let mut has_next = true;
                let mut pagination = db::MAX_DATE.to_string();
                while has_next {
                    let page = match store.get_feeds(pagination.clone()).await {
                        Ok(p) => p,
                        Err(e) => {
                            println!("could not list feeds: {}", e);
                            has_next = false;
                            continue;
                        }
                    };

                    has_next = page.cursor.has_next;
                    pagination = page.cursor.next;

                    let feeds: Vec<Feed> = page.items.iter().map(|r| r.into()).collect();
                    for f in feeds.iter() {
                        match refresh(refresh_store.clone(), f.to_owned()).await {
                            Ok(_) => {}
                            Err(e) => {
                                println!("error updating feed {}: {}", f.feed_url, e);
                            }
                        }
                    }
                }
            });

    future::select(
        Box::pin(serve(routes).run(([0, 0, 0, 0], 8080))),
        Box::pin(refresh_stream),
    )
    .await;
}

#[get("/healthz")]
fn healthz() -> Json<Healthz> {
    Healthz { up: true }.into()
}

#[get("/")]
async fn index(#[data] store: db::Storage) -> Result<ArticleBaseTemplate, Rejection> {
    let page = store
        .get_unread_articles(db::MAX_DATE.to_string())
        .await
        .map_err(reject_anyhow)?;

    Ok(ArticleBaseTemplate {
        title: db::Filter::Unread.to_string(),
        article_filter: db::Filter::Unread.to_string(),
        cursor: page.cursor,
        articles: page.items.iter().map(|r| r.into()).collect(),
    })
}

#[get("/favorites.html")]
async fn favorites(#[data] store: db::Storage) -> Result<ArticleBaseTemplate, Rejection> {
    let page = store
        .get_favorited_articles(db::MAX_DATE.to_string())
        .await
        .map_err(reject_anyhow)?;

    Ok(ArticleBaseTemplate {
        cursor: page.cursor,
        title: "favorites".to_string(),
        article_filter: db::Filter::Favorite.to_string(),
        articles: page.items.iter().map(|r| r.into()).collect(),
    })
}

#[get("/history.html")]
async fn history(#[data] store: db::Storage) -> Result<ArticleBaseTemplate, Rejection> {
    let page = store
        .get_read_articles(db::MAX_DATE.to_string())
        .await
        .map_err(reject_anyhow)?;

    Ok(ArticleBaseTemplate {
        cursor: page.cursor,
        title: "history".to_string(),
        article_filter: db::Filter::Read.to_string(),
        articles: page.items.iter().map(|r| r.into()).collect(),
    })
}

#[get("/feeds.html")]
async fn feeds(#[data] db: db::Storage) -> Result<FeedsTemplate, Rejection> {
    let page = db
        .get_feeds(db::MAX_DATE.to_string())
        .await
        .map_err(reject_anyhow)?;

    Ok(FeedsTemplate {
        cursor: page.cursor,
        feeds: page.items.iter().map(|r| r.into()).collect(),
    })
}

#[get("/add_feed.html")]
async fn add_feed() -> Result<AddFeedTemplate, Rejection> {
    Ok(AddFeedTemplate {})
}

#[post("/feeds")]
async fn create_feed(
    #[form] feed: AddFeed,
    #[data] store: db::Storage,
) -> Result<FeedsTemplate, Rejection> {
    store.add_feed(feed).await.map_err(reject_anyhow)?;
    let page = store
        .get_feeds(db::MAX_DATE.to_string())
        .await
        .map_err(reject_anyhow)?;

    Ok(FeedsTemplate {
        cursor: page.cursor,
        feeds: page.items.iter().map(|r| r.into()).collect(),
    })
}

#[delete("/feeds/{id}")]
async fn delete_feed(
    #[data] store: db::Storage,
    id: String,
    #[header = "pagination"] pagination: String,
) -> Result<FeedListTemplate, Rejection> {
    store.delete_feed(id).await.map_err(reject_anyhow)?;
    let page = store.get_feeds(pagination).await.map_err(reject_anyhow)?;

    Ok(FeedListTemplate {
        cursor: page.cursor,
        feeds: page.items.iter().map(|r| r.into()).collect(),
    })
}

#[post("/feeds/{id}/refresh")]
async fn refresh_feed(
    id: String,
    #[data] store: db::Storage,
    #[header = "pagination"] pagination: String,
) -> Result<FeedListTemplate, Rejection> {
    let f = store
        .get_feed_by_id(id.clone())
        .await
        .map_err(reject_anyhow)?;

    refresh(store.clone(), f).await.map_err(reject_anyhow)?;

    let page = store.get_feeds(pagination).await.map_err(reject_anyhow)?;

    Ok(FeedListTemplate {
        cursor: page.cursor,
        feeds: page.items.iter().map(|r| r.into()).collect(),
    })
}

async fn refresh(store: db::Storage, f: Feed) -> Result<()> {
    let content = reqwest::get(f.feed_url).await?.bytes().await?;

    let parsed_feed = parser::parse(content.reader())?;
    let articles: Vec<Article> = parsed_feed
        .entries
        .iter()
        .map(|e| {
            let mut o: Article = e.into();
            o.feed = f.name.clone();
            o
        })
        .collect();

    store.add_articles(articles.clone().into_iter()).await?;
    store
        .update_feed_last_updated(Article::rfc3339_timestamp(), f.id.clone())
        .await?;

    Ok(())
}

#[post("/articles/{article_id}/read")]
async fn mark_article_read(
    article_id: String,
    #[data] store: db::Storage,
    #[header = "pagination"] pagination: String,
    #[header = "article_filter"] article_filter: String,
) -> Result<ArticleListTemplate, Rejection> {
    let article = store
        .get_article_by_id(article_id.clone())
        .await
        .map_err(reject_anyhow)?;

    store
        .mark_article_read(article)
        .await
        .map_err(reject_anyhow)?;

    let filter = db::Filter::from_str(article_filter.as_str()).map_err(reject_anyhow)?;

    let page = store
        .filter(filter, pagination)
        .await
        .map_err(reject_anyhow)?;

    Ok(ArticleListTemplate {
        cursor: page.cursor,
        articles: page.items.iter().map(|r| r.into()).collect(),
    })
}

#[post("/articles/{article_id}/favorite")]
async fn mark_article_favorite(
    article_id: String,
    #[header = "pagination"] pagination: String,
    #[header = "article_filter"] article_filter: String,
    #[data] store: db::Storage,
) -> Result<ArticleListTemplate, Rejection> {
    store
        .mark_article_favorite(article_id)
        .await
        .map_err(reject_anyhow)?;

    let filter = db::Filter::from_str(article_filter.as_str()).map_err(reject_anyhow)?;

    let page = store
        .filter(filter, pagination)
        .await
        .map_err(reject_anyhow)?;

    Ok(ArticleListTemplate {
        cursor: page.cursor,
        articles: page.items.iter().map(|r| r.into()).collect(),
    })
}

#[get("/articles")]
async fn get_articles(
    #[data] store: db::Storage,
    #[header = "pagination"] pagination: String,
    #[header = "article_filter"] article_filter: String,
) -> Result<ArticleListTemplate, Rejection> {
    let filter = db::Filter::from_str(article_filter.as_str()).map_err(reject_anyhow)?;

    let page = store
        .filter(filter, pagination)
        .await
        .map_err(reject_anyhow)?;

    Ok(ArticleListTemplate {
        cursor: page.cursor,
        articles: page.items.iter().map(|r| r.into()).collect(),
    })
}
