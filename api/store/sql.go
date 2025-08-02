package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type linkRecord struct {
	Id    int64
	Short string
	Url   string
	Desc  string
}

func (lr linkRecord) toGoLink() GoLink {
	return GoLink{
		Id:    lr.Id,
		Short: lr.Short,
		Url:   lr.Url,
		Desc:  lr.Desc,
	}
}

func (lr *linkRecord) scan(row interface{ Scan(...any) error }) error {
	return row.Scan(&lr.Id, &lr.Short, &lr.Url, &lr.Desc)
}

const _table = "golinks"

type SqlStore struct {
	db *sql.DB
}

// NewSqlStore wires a *sql.DB (already opened & ping-tested).
func NewSqlStore(db *sql.DB) (SqlStore, error) {
	if err := createTable(db); err != nil {
		return SqlStore{}, err
	}

	return SqlStore{db: db}, nil
}

// GetLink fetches a single record by short name.
func (s SqlStore) GetLink(ctx context.Context, short string) (GoLink, StoreError) {
	var lr linkRecord
	const q = `SELECT * FROM ` + _table + ` WHERE short = $1`
	if err := lr.scan(s.db.QueryRowContext(ctx, q, short)); err != nil {
		return GoLink{}, wrap(err)
	}
	return lr.toGoLink(), nil
}

func (s SqlStore) CreateLink(ctx context.Context, l GoLink) (int64, StoreError) {
	const q = `INSERT INTO ` + _table + ` (short, url, description) VALUES ($1,$2,$3) RETURNING id`
	var lr linkRecord
	row := s.db.QueryRowContext(ctx, q, l.Short, l.Url, l.Desc)
	if err := row.Scan(&lr.Id); err != nil {
		return 0, &sqlError{fmt.Errorf("failed to create link: %w", err)}
	}

	return lr.Id, nil
}

// UpdateLink overwrites Full, Short & Desc matched on Short.
func (s SqlStore) UpdateLink(ctx context.Context, update LinkUpdate, shortId string) StoreError {
	const q = `
	UPDATE ` + _table + `
	SET short       = COALESCE($1, short),
		url         = COALESCE($2, url),
		description = COALESCE($3, description)
	WHERE short = $4`

	res, err := s.db.ExecContext(ctx, q, update.Short, update.Url, update.Desc, shortId)
	if err != nil {
		return wrap(err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return wrap(err)
	} else if n == 0 {
		return &sqlError{sql.ErrNoRows}
	}

	return nil
}

// DeleteLink removes by short name.
func (s SqlStore) DeleteLink(ctx context.Context, short string) StoreError {
	const q = `DELETE FROM ` + _table + ` WHERE short = $1`
	res, err := s.db.ExecContext(ctx, q, short)
	if err != nil {
		return wrap(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return &sqlError{sql.ErrNoRows}
	}
	return nil
}

func (s SqlStore) ListLinks(ctx context.Context) ([]GoLink, StoreError) {
	const q = `SELECT id, short, url, description FROM ` + _table + ` ORDER BY short`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()

	var out []GoLink
	for rows.Next() {
		var lr linkRecord
		if err := lr.scan(rows); err != nil {
			return nil, wrap(err)
		}
		out = append(out, lr.toGoLink())
	}

	return out, nil
}

type sqlError struct {
	error
}

func wrap(err error) *sqlError {
	if err == nil {
		return nil
	}
	// don't double-wrap
	var sqlErr = new(sqlError)
	if errors.As(err, sqlErr) {
		return sqlErr
	}
	return &sqlError{err}
}

func (err sqlError) Error() string {
	return err.error.Error()
}

func (err sqlError) Unwrap() error {
	return err.error
}

func (err sqlError) IsNotFound() bool {
	return errors.Is(err, sql.ErrNoRows)
}

// our schema is tiny, so we'll just do it all in the application.
func createTable(db *sql.DB) error {
	const q = `
	CREATE TABLE IF NOT EXISTS ` + _table + ` (
		id INTEGER PRIMARY KEY,
		short TEXT NOT NULL UNIQUE,
		url TEXT NOT NULL,
		description TEXT
	);`

	if _, err := db.Exec(q); err != nil {
		return err
	}

	return nil
}
