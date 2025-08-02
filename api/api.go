package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rob-gorman/golinks-dns/api/store"
	"github.com/rob-gorman/golinks-dns/internal/log"
)

// middleware to authorize both our standard read operations,
// and separately, any write/update/delete operations.
type AuthZMiddleware interface {
	Read(http.Handler) http.Handler
	Modify(http.Handler) http.Handler
}

type Api struct {
	db     store.Store
	router http.Handler
	log    log.Logger
}

func NewApi(ctx context.Context,
	db store.Store,
	auth AuthZMiddleware,
	logger log.Logger,
) (Api, error) {

	router := configureRouter(db, auth, logger)
	return Api{
		db:     db,
		router: router,
		log:    logger,
	}, nil
}

func (api Api) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	api.router.ServeHTTP(w, r)
}

// resolves a short link to its full URL
func (api Api) Resolve(ctx context.Context, short string) (full string, err error) {
	if short == "" {
		return "", errors.New("cannot resolve empty short link")
	}

	link, err := api.db.GetLink(ctx, short)
	if err != nil {
		return "", err
	}

	return link.Url, nil
}

func configureRouter(
	db store.Store,
	auth AuthZMiddleware,
	log log.Logger,
) http.Handler {

	var (
		read   = auth.Read
		modify = auth.Modify
		router = http.NewServeMux()
	)

	router.Handle("GET /links", read(handleListLinks(db, log)))
	router.Handle("GET /links/{id}", read(handleGetLink(db, log)))

	router.Handle("POST /links", modify(handleCreateLink(db, log)))
	router.Handle("PATCH /links/{id}", modify(handleUpdateLink(db, log)))
	router.Handle("DELETE /links/{id}", modify(handleDeleteLink(db, log)))

	return router
}

func handleGetLink(db store.Store, log log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		short := r.PathValue("id")
		link, err := db.GetLink(r.Context(), short)
		if err != nil {
			if err.IsNotFound() {
				serveError(w, "link not found", http.StatusNotFound)
				return
			}
			log.Errorw("failed to get link", "error", err)
			serveError(w, "failed to get link", http.StatusInternalServerError)
			return
		}

		resp := map[string]any{
			"data": link,
		}

		serveResponse(w, resp, log)
	}
}

func handleCreateLink(db store.Store, log log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		link, err := parseRequest[store.GoLink](r)
		if err != nil {
			log.Errorw("failed to parse request", "error", err)
			serveError(w, "error encountered parsing request", http.StatusBadRequest)
			return
		}

		created, err := db.CreateLink(r.Context(), link)
		if err != nil {
			log.Errorw("failed to create link", "error", err, "link", link)
			serveError(w, "failed to create link", http.StatusInternalServerError)
			return
		}

		resp := map[string]any{
			"data": map[string]any{
				"id": created,
			},
		}

		serveResponse(w, resp, log)
	}
}

func handleUpdateLink(db store.Store, log log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		update, err := parseRequest[store.LinkUpdate](r)
		if err != nil {
			log.Errorw("failed to parse request", "error", err)
			serveError(w, "error encountered parsing request", http.StatusBadRequest)
			return
		}

		short := r.PathValue("id")
		qErr := db.UpdateLink(r.Context(), update, short)
		if qErr != nil {
			if qErr.IsNotFound() {
				serveError(w, "link not found", http.StatusNotFound)
				return
			}
			log.Errorw("failed to update link", "error", qErr)
			serveError(w, "failed to update link", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func handleDeleteLink(db store.Store, log log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		short := r.PathValue("id")
		err := db.DeleteLink(r.Context(), short)
		if err != nil {
			if err.IsNotFound() {
				serveError(w, "link not found", http.StatusNotFound)
				return
			}
			log.Errorw("failed to delete link", "error", err)
			serveError(w, "failed to delete link", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleListLinks(db store.Store, log log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		links, err := db.ListLinks(r.Context())
		if err != nil {
			log.Errorw("failed to list links", "error", err)
			serveError(w, "failed to list links", http.StatusInternalServerError)
			return
		}

		// no pagination for now
		response := map[string]any{
			"data":  links,
			"total": len(links),
		}

		serveResponse(w, response, log)
	}
}

func serveResponse(w http.ResponseWriter, resp map[string]any, log log.Logger) {
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Errorw("failed to encode response", "error", err)
		serveError(w, "the server encountered an error", http.StatusInternalServerError)
		return
	}
}

// we may have a fancier error page one day
func serveError(w http.ResponseWriter, msg string, status int) {
	http.Error(w, msg, status)
}

func parseRequest[T interface{ Validate() error }](r *http.Request) (T, error) {
	var t T
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		return t, err
	}
	if err := t.Validate(); err != nil {
		return t, err
	}
	return t, nil
}
