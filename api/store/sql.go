// I haven't worked with SQL in years and so this is pretty naive and sad.
// For a more complex data model, I'd probably split the default Store implementation
// to its own package and manage type definitions accordingly.
package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrNoRecord = errors.New("no record found")

// internal db representation of a link; not all that necessary tbh
type linkRecord struct {
	Id       int64
	Short    string
	Url      string
	Desc     string
	Created  time.Time
	Accessed *sql.NullTime // TODO
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
	return row.Scan(&lr.Id, &lr.Short, &lr.Url, &lr.Desc, &lr.Created, &lr.Accessed)
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
func (s SqlStore) GetLink(ctx context.Context, short string) (GoLink, error) {
	var lr linkRecord
	const q = `SELECT * FROM ` + _table + ` WHERE short = $1`
	if err := lr.scan(s.db.QueryRowContext(ctx, q, short)); err != nil {
		return GoLink{}, err
	}
	return lr.toGoLink(), nil
}

func (s SqlStore) CreateLink(ctx context.Context, l GoLink) (GoLink, error) {
	const q = `INSERT INTO ` + _table + ` (short, url, description) VALUES ($1,$2,$3) RETURNING *`
	var lr linkRecord
	row := s.db.QueryRowContext(ctx, q, l.Short, l.Url, l.Desc)
	if err := lr.scan(row); err != nil {
		return GoLink{}, err
	}

	return lr.toGoLink(), nil
}

// UpdateLink overwrites Full, Short & Desc matched on Short.
func (s SqlStore) UpdateLink(ctx context.Context, update LinkUpdate, shortId string) error {
	const q = `
	UPDATE ` + _table + `
	SET short       = COALESCE($1, short),
		url         = COALESCE($2, url),
		description = COALESCE($3, description),
		accessed    = CURRENT_TIMESTAMP
	WHERE short = $4`

	res, err := s.db.ExecContext(ctx, q, update.Short, update.Url, update.Desc, shortId)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return ErrNoRecord
	}

	return nil
}

// DeleteLink removes by short name.
func (s SqlStore) DeleteLink(ctx context.Context, short string) error {
	const q = `DELETE FROM ` + _table + ` WHERE short = $1`
	res, err := s.db.ExecContext(ctx, q, short)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoRecord
	}
	return nil
}

// No limit for pagination; probably fine
func (s SqlStore) ListLinks(ctx context.Context) ([]GoLink, error) {
	const q = `SELECT * FROM ` + _table + ` ORDER BY short`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []GoLink
	for rows.Next() {
		var lr linkRecord
		if err := lr.scan(rows); err != nil {
			return nil, err
		}
		out = append(out, lr.toGoLink())
	}

	return out, nil
}

// our schema is tiny, so we'll just do it all in the application.
func createTable(db *sql.DB) error {
	const q = `
	CREATE TABLE IF NOT EXISTS ` + _table + ` (
		id INTEGER PRIMARY KEY,
		short TEXT NOT NULL UNIQUE,
		url TEXT NOT NULL,
		description TEXT,
		created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		accessed TIMESTAMP
	);`

	if _, err := db.Exec(q); err != nil {
		return err
	}

	return nil
}
