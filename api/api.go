package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/rob-gorman/golinks/api/store"
	"github.com/rob-gorman/golinks/internal/logger"
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
	log    logger.Logger
}

func NewApi(ctx context.Context,
	db store.Store,
	auth AuthZMiddleware,
	logger logger.Logger,
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

// as the arbiter of our data, the API itself is responsible for resolving a short link to its full URL
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
	log logger.Logger,
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

type oneResponse struct {
	Data      store.GoLink `json:"data"`
	Status    int          `json:"status"`
	Error     string       `json:"error,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
}

func (r oneResponse) status() int {
	return r.Status
}

func handleGetLink(db store.Store, log logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		short := r.PathValue("id")
		link, err := db.GetLink(r.Context(), short)
		if err != nil {
			if store.IsNotFound(err) {
				serveError(w, apiError(err, http.StatusNotFound), log)
				return
			}
			log.Error("failed to get link", "error", err)
			serveError(w, apiError(err, http.StatusInternalServerError), log)
			return
		}

		resp := oneResponse{
			Data:      link,
			Status:    http.StatusOK,
			Timestamp: time.Now(),
		}

		serveResponse(w, resp, log)
	}
}

func handleCreateLink(db store.Store, log logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		link, err := parseRequest[store.GoLink](r)
		if err != nil {
			log.Error("failed to parse request", "error", err)
			serveError(w, apiError(err, http.StatusBadRequest), log)
			return
		}

		created, err := db.CreateLink(r.Context(), link)
		if err != nil {
			log.Error("failed to create link", "error", err, "link", link)
			serveError(w, apiError(err, http.StatusInternalServerError), log)
			return
		}

		resp := oneResponse{
			Data:      created,
			Status:    http.StatusCreated,
			Timestamp: time.Now(),
		}

		serveResponse(w, resp, log)
	}
}

func handleUpdateLink(db store.Store, log logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		update, err := parseRequest[store.LinkUpdate](r)
		if err != nil {
			log.Error("failed to parse request", "error", err)
			serveError(w, apiError(err, http.StatusBadRequest), log)
			return
		}

		short := r.PathValue("id")
		qErr := db.UpdateLink(r.Context(), update, short)
		if qErr != nil {
			if store.IsNotFound(qErr) {
				serveError(w, apiError(qErr, http.StatusNotFound), log)
				return
			}
			log.Error("failed to update link", "error", qErr)
			serveError(w, apiError(qErr, http.StatusInternalServerError), log)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func handleDeleteLink(db store.Store, log logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		short := r.PathValue("id")
		err := db.DeleteLink(r.Context(), short)
		if err != nil {
			if store.IsNotFound(err) {
				serveError(w, apiError(err, http.StatusNotFound), log)
				return
			}
			log.Error("failed to delete link", "error", err)
			serveError(w, apiError(err, http.StatusInternalServerError), log)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type listResponse struct {
	Data      []store.GoLink `json:"data,omitempty"`
	Status    int            `json:"status"`
	Timestamp time.Time      `json:"timestamp"`
	Total     int            `json:"total"`
}

func (r listResponse) status() int {
	return r.Status
}

func handleListLinks(db store.Store, log logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		links, err := db.ListLinks(r.Context())
		if err != nil {
			log.Error("failed to list links", "error", err)
			serveError(w, apiError(err, http.StatusInternalServerError), log)
			return
		}

		// no pagination for now
		response := listResponse{
			Data:      links,
			Status:    http.StatusOK,
			Timestamp: time.Now(),
			Total:     len(links),
		}

		serveResponse(w, response, log)
	}
}

type response interface {
	status() int
}

func serveResponse[T response](w http.ResponseWriter, resp T, log logger.Logger) {
	w.WriteHeader(resp.status())
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error("failed to encode response", "error", err)
		// redundant status header write here
		serveError(w, apiError(err, http.StatusInternalServerError), log)
		return
	}
}

type errorResponse struct {
	Error     string    `json:"error"`
	Status    int       `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// we do want to return the raw backend error here for our use case.
func apiError(err error, status int) errorResponse {
	return errorResponse{
		Error:     err.Error(),
		Status:    status,
		Timestamp: time.Now(),
	}
}

func serveError(w http.ResponseWriter, err errorResponse, log logger.Logger) {
	w.WriteHeader(err.Status)
	if err := json.NewEncoder(w).Encode(err); err != nil {
		log.Error("failed to encode error", "error", err)
	}
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
