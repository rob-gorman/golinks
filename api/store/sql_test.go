package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDb(t *testing.T, file ...string) *sql.DB {
	dbPath := ":memory:"
	if len(file) > 0 {
		dbPath = file[0]
	}
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)

	return db
}

func TestSqlStore_CRUD(t *testing.T) {
	ctx := t.Context()
	db := testDb(t)
	store, err := NewSqlStore(db)
	require.NoError(t, err)

	link := GoLink{
		Short: "test",
		Url:   "https://example.com/test",
		Desc:  "A test link",
	}

	created := link
	t.Run("create", func(t *testing.T) {
		id, err := store.CreateLink(ctx, link)
		require.NoError(t, err)
		require.NotZero(t, id)

		created.Id = id
	})

	t.Run("get", func(t *testing.T) {
		fetched, err := store.GetLink(ctx, "test")
		require.NoError(t, err)
		require.NotEmpty(t, fetched.Id)
		assert.Equal(t, created, fetched)
	})

	t.Run("get all", func(t *testing.T) {
		all, err := store.ListLinks(ctx)
		require.NoError(t, err)
		assert.Equal(t, []GoLink{created}, all)
	})

	t.Run("update", func(t *testing.T) {
		desc := "Updated description"
		update := LinkUpdate{Desc: &desc}

		err := store.UpdateLink(ctx, update, created.Short)
		require.NoError(t, err)

		fetched, err := store.GetLink(ctx, created.Short)
		require.NoError(t, err)

		assert.Equal(t, created.Short, fetched.Short)
		assert.NotEqual(t, created.Desc, fetched.Desc)
		assert.Equal(t, desc, fetched.Desc)
	})

	t.Run("delete", func(t *testing.T) {
		err = store.DeleteLink(ctx, created.Short)
		require.NoError(t, err)

		_, err = store.GetLink(ctx, created.Short)
		require.Error(t, err)
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})
}

func TestSqlStore_Persistence(t *testing.T) {
	ctx := t.Context()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	// 1. Create DB and a record
	db1 := testDb(t, dbPath)
	store1, err := NewSqlStore(db1)
	require.NoError(t, err)

	link := GoLink{
		Short: "persist",
		Url:   "https://example.com/persist",
		Desc:  "This should be saved to disk.",
	}
	createdId, err := store1.CreateLink(ctx, link)
	require.NoError(t, err)
	require.NoError(t, db1.Close())

	// 2. Re-open DB and verify record exists
	db2 := testDb(t, dbPath)
	t.Cleanup(func() {
		require.NoError(t, db2.Close())
	})
	store2, err := NewSqlStore(db2)
	require.NoError(t, err)

	fetched, err := store2.GetLink(ctx, "persist")
	require.NoError(t, err)
	assert.Equal(t, createdId, fetched.Id)
}

func TestSqlError(t *testing.T) {
	var err StoreError = &sqlError{sql.ErrNoRows}
	assert.True(t, err.IsNotFound())
	assert.ErrorIs(t, err, sql.ErrNoRows)
}
