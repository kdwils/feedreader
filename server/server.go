package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/araddon/dateparse"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/kdwils/feedreader/pkg/parser"
	"github.com/kdwils/feedreader/storage"
	"go.uber.org/zap"
)

type Server struct {
	Storage storage.Storage
	logger  *zap.Logger
	parser  parser.Parser
}

func New(Storage storage.Storage, parser parser.Parser, logger *zap.Logger) Server {
	return Server{
		Storage: Storage,
		parser:  parser,
		logger:  logger,
	}
}

func writeResponse(w http.ResponseWriter, status int, body interface{}) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	w.Write(b)
	return nil
}

func (s Server) Serve(port int) {
	rtr := mux.NewRouter()
	rtr.Use(s.LogMiddleware())

	rtr.HandleFunc("/api/feeds", s.CreateFeed()).Methods(http.MethodPost)
	rtr.HandleFunc("/api/feeds", s.OptionsMiddleware(s.ListFeeds())).Methods(http.MethodGet)

	rtr.HandleFunc("/api/articles", s.CreateArticle()).Methods(http.MethodPost)
	rtr.HandleFunc("/api/articles", s.OptionsMiddleware(s.ListArticles())).Methods(http.MethodGet)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handlers.CORS()(rtr),
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Fatal("failed to serve", zap.Error(err))
		}
	}()

	s.logger.Info("serving", zap.Int("port", port))
	<-done

	s.logger.Info("stopping server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()

	if err := srv.Shutdown(ctx); err != nil {
		s.logger.Fatal("server shutdown failed", zap.Error(err))
	}
}

func (s Server) CreateFeed() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := LoggerFromContext(r.Context())

		b, err := io.ReadAll(r.Body)
		if err != nil {
			l.Error("failed to parse request body")
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var request storage.CreateFeedRequest
		err = json.Unmarshal(b, &request)
		if err != nil {
			l.Error("failed to unmarshal request body", zap.Error(err), zap.ByteString("body", b))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		parsedFeed, err := s.parser.ParseFromURI(r.Context(), request.Link)
		if err != nil {
			l.Error("failed to parse feed", zap.String("link", request.Link), zap.Error(err))
			http.Error(w, "failed to parse feed", http.StatusInternalServerError)
			return
		}

		log.Printf("%+v", parsedFeed.Channel)

		feed, err := s.Storage.CreateFeed(r.Context(), parsedFeed.Channel.Title, request.Link, parsedFeed.Channel.Link, parsedFeed.Channel.Description)
		if err != nil {
			l.Error("failed to create feed", zap.Error(err), zap.Any("request", request))
			http.Error(w, "failed to create feed", http.StatusBadRequest)
			return
		}

		writeResponse(w, http.StatusCreated, feed)
	}
}

func (s Server) ListFeeds() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		opts := OptionsFromContext(r.Context())
		l := LoggerFromContext(r.Context(), zap.Any("options", opts))
		feeds, err := s.Storage.ListFeeds(r.Context(), opts)
		if err != nil {
			l.Error("failed to list feeds", zap.Error(err))
			http.Error(w, "failed to list feeds", http.StatusInternalServerError)
			return
		}

		writeResponse(w, http.StatusOK, feeds)
	}
}

func (s Server) CreateArticle() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := LoggerFromContext(r.Context())

		b, err := io.ReadAll(r.Body)
		if err != nil {
			l.Error("failed to parse request body")
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var request storage.CreateArticleRequest
		err = json.Unmarshal(b, &request)
		if err != nil {
			l.Error("failed to unmarshal request body", zap.Error(err), zap.ByteString("body", b))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		publishedTime, err := dateparse.ParseAny(request.Published)
		if err != nil {
			l.Error("failed to parse article published date", zap.Error(err), zap.String("timestamp", request.Published))
			http.Error(w, "failed to parse article published date", http.StatusBadRequest)
			return
		}

		article, err := s.Storage.CreateArticle(r.Context(), request.Link, request.Title, request.Author, request.Description, publishedTime)
		if err != nil {
			l.Error("failed to create article", zap.Error(err), zap.Any("request", request))
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		writeResponse(w, http.StatusCreated, article)
	}
}

func (s Server) ListArticles() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		opts := OptionsFromContext(r.Context())
		l := LoggerFromContext(r.Context(), zap.Any("options", opts))
		articles, err := s.Storage.ListArticles(r.Context(), opts)
		if err != nil {
			l.Error("failed to list articles", zap.Error(err))
			http.Error(w, "failed to list articles", http.StatusInternalServerError)
			return
		}

		writeResponse(w, http.StatusOK, articles)
	}
}
