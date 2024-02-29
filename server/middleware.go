package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/kdwils/feedreader/storage"
	"go.uber.org/zap"
)

func (s Server) LogMiddleware() mux.MiddlewareFunc {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(LoggerToContext(r.Context(), s.logger.With(zap.String("path", r.URL.Path))))
			h.ServeHTTP(w, r)
		})
	}
}

func (s Server) OptionsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		opts := storage.ParseOptions(r.URL.Query())
		r = r.WithContext(OptionsToContext(r.Context(), opts))
		next(w, r)
	})
}
