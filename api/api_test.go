package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rob-gorman/golinks-dns/api/store"
	"github.com/rob-gorman/golinks-dns/internal/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func TestAPI_CRUD(t *testing.T) {
	api := testApi(t)
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)

	// these subtests are just distinguished for convenience; they
	// very much are not isolated from each other.
	var createdLink store.GoLink

	t.Run("create", func(t *testing.T) {
		linkPayload := store.GoLink{
			Short: "test-api",
			Url:   "https://example.com/api-test",
			Desc:  "An API test link",
		}
		body, _ := json.Marshal(linkPayload)
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/links", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var respBody map[string]map[string]any
		err = json.NewDecoder(resp.Body).Decode(&respBody)
		require.NoError(t, err)

		idFloat, ok := respBody["data"]["id"].(float64)
		require.True(t, ok, "ID was not a number")

		createdLink = linkPayload
		createdLink.Id = int64(idFloat)
		require.NotZero(t, createdLink.Id)
	})

	t.Run("get", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, server.URL+"/links/"+createdLink.Short, nil)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var respBody map[string]store.GoLink
		err = json.NewDecoder(resp.Body).Decode(&respBody)
		require.NoError(t, err)

		assert.Equal(t, createdLink, respBody["data"])
	})

	t.Run("list", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, server.URL+"/links", nil)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var respBody = make(map[string]any)
		err = json.NewDecoder(resp.Body).Decode(&respBody)
		require.NoError(t, err)

		assert.Equal(t, 1.0, respBody["total"]) // json unmarshals numbers to float64

		// Re-marshal and unmarshal data part to get a comparable struct
		dataBytes, _ := json.Marshal(respBody["data"])
		var fetchedLinks []store.GoLink
		err = json.Unmarshal(dataBytes, &fetchedLinks)
		require.NoError(t, err)

		assert.Equal(t, []store.GoLink{createdLink}, fetchedLinks)
	})

	t.Run("update", func(t *testing.T) {
		desc := "An updated API test link"
		updatePayload := store.LinkUpdate{Desc: &desc}
		body, _ := json.Marshal(updatePayload)
		req, _ := http.NewRequest(http.MethodPatch, server.URL+"/links/"+createdLink.Short, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	t.Run("delete", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, server.URL+"/links/"+createdLink.Short, nil)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

		req, _ = http.NewRequest(http.MethodGet, server.URL+"/links/"+createdLink.Short, nil)
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

// testApi sets up an in-memory DB and a new API instance for testing.
func testApi(t *testing.T) *Api {
	t.Helper()

	dbConn, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { dbConn.Close() })

	store, err := store.NewSqlStore(dbConn)
	require.NoError(t, err)

	logger := log.Default()

	auth := new(noopMW)

	api, err := NewApi(t.Context(), store, auth, logger)
	require.NoError(t, err)

	return &api
}

type noopMW struct{} // skip authorization

func (m noopMW) Read(next http.Handler) http.Handler   { return next }
func (m noopMW) Modify(next http.Handler) http.Handler { return next }
