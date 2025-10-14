package redirector

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/rob-gorman/golinks/api/store"
	"github.com/rob-gorman/golinks/internal/logger"
	"github.com/stretchr/testify/require"
)

var _testLinks = []store.GoLink{
	{
		Short: "test",
		Url:   "https://example.com/test",
		Desc:  "A test link",
	},
	{
		Short: "yt",
		Url:   "https://youtube.com",
		Desc:  "A youtube link",
	},
	{
		Short: "gh",
		Url:   "https://github.com",
		Desc:  "A github link",
	},
	{
		Short: "htmx",
		Url:   "https://htmx.org/docs/#installing",
		Desc:  "htmx docs",
	},
}

func testDb(t *testing.T) store.Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})
	store, err := store.NewSqlStore(db)
	require.NoError(t, err)
	for _, link := range _testLinks {
		_, err := store.CreateLink(t.Context(), link)
		require.NoError(t, err)
	}
	return store
}

func testClient(t *testing.T) *http.Client {
	t.Helper()
	return &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func TestRedirector(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	db := testDb(t)
	client := testClient(t)
	log := logger.Default()
	port := 3007
	addr := fmt.Sprintf("http://127.0.0.1:%d", port)

	go func() {
		if err := Run(ctx, db, log, WithPort(port)); err != nil {
			t.Errorf("failed to start redirector: %v", err)
		}
	}()

	time.Sleep(200 * time.Millisecond)

	t.Run("redirect", func(t *testing.T) {
		for _, link := range _testLinks {
			t.Run(link.Desc, func(t *testing.T) {
				url := fmt.Sprintf("%s/%s", addr, link.Short)
				req, err := http.NewRequest(http.MethodGet, url, nil)
				require.NoError(t, err)
				resp, err := client.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)
				require.Equal(t, link.Url, resp.Header.Get("Location"))
			})
		}
	})
}

func TestHandler(t *testing.T) {
	t.Parallel()

	db := testDb(t)
	h := configureRouter(db, logger.Default())

	t.Run("redirect", func(t *testing.T) {
		for _, link := range _testLinks {
			t.Run(link.Desc, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/"+link.Short, nil)
				rr := httptest.NewRecorder()
				h.ServeHTTP(rr, req)

				require.Equal(t, http.StatusTemporaryRedirect, rr.Code)
				require.Equal(t, link.Url, rr.Header().Get("Location"))
			})
		}
	})

	t.Run("no link", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		require.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("malformed", func(t *testing.T) {
		for _, path := range []string{"/", "/foo/bar"} {
			t.Run(path, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, path, nil)
				rr := httptest.NewRecorder()
				h.ServeHTTP(rr, req)
				require.Equal(t, http.StatusBadRequest, rr.Code)
			})
		}
	})
}
