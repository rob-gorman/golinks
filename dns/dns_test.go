package dns

import (
	"database/sql"
	"testing"

	"github.com/rob-gorman/golinks/api/store"
	"github.com/rob-gorman/golinks/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func testDb(t *testing.T) store.Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	store, err := store.NewSqlStore(db)
	require.NoError(t, err)
	return store
}

func TestDefaultResolver_Resolve_FromSQLite(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	db := testDb(t)

	// Seed one record
	link := store.GoLink{Short: "foo", Url: "https://example.com/foo", Desc: "desc"}
	created, err := db.CreateLink(ctx, link)
	require.NoError(t, err)
	require.NotZero(t, created.Id)

	r := DefaultResolver(db, logger.Default())

	url, err := r.Resolve(ctx, "foo")
	require.NoError(t, err)
	assert.Equal(t, link.Url, url)
}

func TestDefaultResolver_NotFound(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	db := testDb(t)

	r := DefaultResolver(db, logger.Default())

	url, err := r.Resolve(ctx, "does-not-exist")
	require.Error(t, err)
	assert.Empty(t, url)
}
