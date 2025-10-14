package redirector

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/rob-gorman/golinks/api/store"
	"github.com/rob-gorman/golinks/internal/logger"
	"golang.org/x/sync/errgroup"
)

type linkStore interface {
	GetLink(context.Context, string) (store.GoLink, error)
}

type server struct {
	srv *http.Server
	db  linkStore
}

// configures and runs the underlying HTTP server. Default addr is ":http". This blocks.
func Run(ctx context.Context, db linkStore, log logger.Logger, opts ...Option) error {
	config := new(config)

	for _, opt := range opts {
		opt(config)
	}

	s := &server{
		srv: &http.Server{
			Addr:    config.addr, // stdlib default is ":http"
			Handler: configureRouter(db, log),
		},
		db: db,
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	g.Go(func() error {
		<-ctx.Done()
		sdCtx, sdCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer sdCancel()
		return s.srv.Shutdown(sdCtx)
	})

	return g.Wait()
}

func configureRouter(db linkStore, log logger.Logger) http.Handler {
	router := http.NewServeMux()
	router.Handle("/{short}", handleRedirect(db, log))
	router.Handle("/", http.HandlerFunc(handleOther))
	return router
}

func handleRedirect(db linkStore, log logger.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		short := r.PathValue("short")
		link, err := db.GetLink(r.Context(), short)
		if err != nil {
			if store.IsNotFound(err) {
				serveError(w, fmt.Errorf("link %q not found: %w", short, err), http.StatusNotFound)
				return
			}
			log.Error("failed to get link", "error", err)
			serveError(w, fmt.Errorf("db query failed: %w", err), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, link.Url, http.StatusTemporaryRedirect)
	})
}

func handleOther(w http.ResponseWriter, r *http.Request) {
	err := fmt.Errorf("could not resolve request: %s", r.URL.String())
	serveError(w, err, http.StatusBadRequest)
}

func serveError(w http.ResponseWriter, err error, status int) {
	w.WriteHeader(status)
	w.Write([]byte(err.Error()))
}

type Option func(*config)

func WithPort(port int) Option {
	return func(c *config) {
		c.addr = fmt.Sprintf(":%d", port)
	}
}

type config struct {
	addr string
}
