package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rob-gorman/golinks/api/store"
	"github.com/rob-gorman/golinks/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func TestAPI_CRUD(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	api := makeTestApi(t)
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)

	// these subtests are just distinguished for convenience; they
	// very much are not isolated from each other.
	var createdLink store.GoLink

	t.Run("create", func(t *testing.T) {
		payload := store.GoLink{
			Short: "test-api",
			Url:   "https://example.com/api-test",
			Desc:  "An API test link",
		}
		body, err := json.Marshal(payload)
		require.NoError(t, err)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/links", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var respBody oneResponse
		err = json.NewDecoder(resp.Body).Decode(&respBody)
		require.NoError(t, err)

		createdLink = respBody.Data
		require.NotZero(t, createdLink.Id)
		assert.Equal(t, payload.Short, createdLink.Short)
	})

	t.Run("get", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/links/"+createdLink.Short, nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var respBody oneResponse
		err = json.NewDecoder(resp.Body).Decode(&respBody)
		require.NoError(t, err)

		assert.Equal(t, createdLink, respBody.Data)
	})

	t.Run("list", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/links", nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var respBody listResponse
		err = json.NewDecoder(resp.Body).Decode(&respBody)
		require.NoError(t, err)

		assert.Equal(t, 1, respBody.Total)

		assert.Equal(t, []store.GoLink{createdLink}, respBody.Data)
	})

	t.Run("update", func(t *testing.T) {
		desc := "An updated API test link"
		updatePayload := store.LinkUpdate{Desc: &desc}
		body, err := json.Marshal(updatePayload)
		require.NoError(t, err)
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPatch, server.URL+"/links/"+createdLink.Short, bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	t.Run("delete", func(t *testing.T) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, server.URL+"/links/"+createdLink.Short, nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

		req, err = http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/links/"+createdLink.Short, nil)
		require.NoError(t, err)
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestHandlers(t *testing.T) {
	t.Parallel()

	logger := logger.Default()

	type testCase struct {
		name    string
		link    store.GoLink
		invalid bool
	}

	tests := []testCase{
		{
			name: "valid-https-url",
			link: store.GoLink{
				Short: "test-handler",
				Url:   "https://example.com/handler-test",
				Desc:  "A handler test link",
			},
		},
		{
			name: "valid-http-url",
			link: store.GoLink{
				Short: "test-handler-2",
				Url:   "http://example.dev/handler-test-2",
				Desc:  "A handler test link 2",
			},
		},
		{
			name: "valid-s3-url",
			link: store.GoLink{
				Short: "test-handler-3",
				Url:   "s3://my-bucket/my-prefix/my-object",
				Desc:  "A handler test link 3",
			},
		},
		{
			name: "invalid-short",
			link: store.GoLink{
				Short: "",
				Url:   "https://example.com/invalid-short",
				Desc:  "An invalid short link",
			},
			invalid: true,
		},
		{
			name: "invalid-url",
			link: store.GoLink{
				Short: "valid-short-9",
				Url:   "invalid-url",
				Desc:  "An invalid URL link",
			},
			invalid: true,
		},
		{
			name: "invalid-empty-url",
			link: store.GoLink{
				Short: "test-handler-4",
				Url:   "",
				Desc:  "An invalid URL link",
			},
			invalid: true,
		},
		{
			name: "invalid-empty-both",
			link: store.GoLink{
				Short: "",
				Url:   "",
				Desc:  "An invalid short and URL link",
			},
			invalid: true,
		},
	}

	// returns slice of tests with hydrated link values
	insertValid := func(t *testing.T, db store.Store) []testCase {
		t.Helper()

		inserted := make([]testCase, 0, len(tests))
		for _, tc := range tests {
			if tc.invalid {
				continue
			}
			created, err := db.CreateLink(t.Context(), tc.link)
			require.NoError(t, err)
			tc.link = created
			inserted = append(inserted, tc)
		}
		return inserted
	}

	t.Run("create", func(t *testing.T) {
		db := makeTestStore(t)
		handler := handleCreateLink(db, logger)

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				body, err := json.Marshal(tt.link)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "/links", bytes.NewReader(body))
				rr := httptest.NewRecorder()

				handler.ServeHTTP(rr, req)

				if tt.invalid {
					assert.Equal(t, http.StatusBadRequest, rr.Code)
					var respBody errorResponse
					err = json.NewDecoder(rr.Body).Decode(&respBody)
					require.NoError(t, err)
					assert.Equal(t, respBody.Status, rr.Code)
					return
				}

				assert.Equal(t, http.StatusCreated, rr.Code)

				var respBody oneResponse
				err = json.NewDecoder(rr.Body).Decode(&respBody)
				require.NoError(t, err)

				assert.Equal(t, tt.link.Short, respBody.Data.Short)
			})
		}
	})

	t.Run("get", func(t *testing.T) {
		db := makeTestStore(t)
		handler := handleGetLink(db, logger)

		inserted := insertValid(t, db)

		for _, tt := range inserted {
			link := tt.link
			short := link.Short
			t.Run(short, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/links/"+short, nil)
				req.SetPathValue("id", short)
				rr := httptest.NewRecorder()

				handler.ServeHTTP(rr, req)

				assert.Equal(t, http.StatusOK, rr.Code)

				var respBody oneResponse
				err := json.NewDecoder(rr.Body).Decode(&respBody)
				require.NoError(t, err)

				assert.Equal(t, link, respBody.Data)
			})
		}
	})

	t.Run("list", func(t *testing.T) {
		db := makeTestStore(t)
		inserted := insertValid(t, db)

		handler := handleListLinks(db, logger)
		req := httptest.NewRequest(http.MethodGet, "/links", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var respBody listResponse
		err := json.NewDecoder(rr.Body).Decode(&respBody)
		require.NoError(t, err)

		assert.Equal(t, len(inserted), respBody.Total)
		var insertedLinks []store.GoLink
		for _, link := range inserted {
			insertedLinks = append(insertedLinks, link.link)
		}
		assert.ElementsMatch(t, insertedLinks, respBody.Data)
	})

	t.Run("update", func(t *testing.T) {
		db := makeTestStore(t)
		inserted := insertValid(t, db)
		handler := handleUpdateLink(db, logger)

		t.Run("valid", func(t *testing.T) {
			// same update for all
			desc := "An updated handler test link"
			payload := store.LinkUpdate{Desc: &desc}
			body, err := json.Marshal(payload)
			require.NoError(t, err)

			for _, tt := range inserted {
				link := tt.link
				short := link.Short
				t.Run(tt.name, func(t *testing.T) {
					req := httptest.NewRequest(http.MethodPatch, "/links/"+short, bytes.NewReader(body))
					req.SetPathValue("id", short)
					rr := httptest.NewRecorder()

					handler.ServeHTTP(rr, req)
					assert.Equal(t, http.StatusNoContent, rr.Code)

					// assert updated
					modified, err := db.GetLink(t.Context(), short)
					require.NoError(t, err)
					assert.Equal(t, desc, modified.Desc)
				})
			}
		})

		t.Run("invalid", func(t *testing.T) {
			url := "invalid-url"
			payload := store.LinkUpdate{Url: &url}
			body, err := json.Marshal(payload)
			require.NoError(t, err)

			for _, tt := range inserted {
				short := tt.link.Short
				t.Run(short, func(t *testing.T) {
					req := httptest.NewRequest(http.MethodPatch, "/links/"+short, bytes.NewReader(body))
					req.SetPathValue("id", short)
					rr := httptest.NewRecorder()

					handler.ServeHTTP(rr, req)

					assert.Equal(t, http.StatusBadRequest, rr.Code)
				})
			}
		})
	})

	t.Run("delete", func(t *testing.T) {
		db := makeTestStore(t)
		inserted := insertValid(t, db)

		handler := handleDeleteLink(db, logger)
		for _, tt := range inserted {
			short := tt.link.Short
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodDelete, "/links/"+short, nil)
				req.SetPathValue("id", short)
				rr := httptest.NewRecorder()

				handler.ServeHTTP(rr, req)
				assert.Equal(t, http.StatusNoContent, rr.Code)

				// Verify it's gone
				getHandler := handleGetLink(db, logger)
				req = httptest.NewRequest(http.MethodGet, "/links/"+short, nil)
				rr = httptest.NewRecorder()

				getHandler.ServeHTTP(rr, req)
				assert.Equal(t, http.StatusNotFound, rr.Code)
			})
		}
	})
}

func makeTestStore(t *testing.T) store.Store {
	t.Helper()

	dbConn, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { dbConn.Close() })

	db, err := store.NewSqlStore(dbConn)
	require.NoError(t, err)
	return db
}

// testApi sets up an in-memory DB and a new API instance for testing.
func makeTestApi(t *testing.T) *Api {
	t.Helper()

	db := makeTestStore(t)

	logger := logger.Default()

	auth := new(noopMW)

	api, err := NewApi(t.Context(), db, auth, logger)
	require.NoError(t, err)

	return &api
}

type noopMW struct{} // skip authorization

func (m noopMW) Read(next http.Handler) http.Handler   { return next }
func (m noopMW) Modify(next http.Handler) http.Handler { return next }
