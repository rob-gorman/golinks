package store

import (
	"context"
	"errors"
	"fmt"
	"net/url"
)

type Store interface {
	GetLink(context.Context, string) (GoLink, StoreError)
	CreateLink(context.Context, GoLink) (GoLink, StoreError) // maybe return just ID
	UpdateLink(context.Context, LinkUpdate, string) StoreError
	DeleteLink(context.Context, string) StoreError
	ListLinks(context.Context) ([]GoLink, StoreError)
}

type GoLink struct {
	Id    int64  `json:"id,omitempty"`
	Short string `json:"short,omitempty"`
	Url   string `json:"url,omitempty"`
	Desc  string `json:"desc,omitempty"`
}

func (l GoLink) Validate() error {
	if l.Short == "" {
		return errors.New("short link cannot be empty")
	}
	if err := validateUrl(l.Url); err != nil {
		return fmt.Errorf("full url must be a valid URL: %w", err)
	}
	return nil
}

func validateUrl(rawUrl string) error {
	if rawUrl == "" {
		return errors.New("url cannot be empty")
	}

	u, err := url.Parse(rawUrl)
	if err != nil {
		return err
	}

	if u.Scheme == "" || u.Host == "" {
		return errors.New("url must be absolute")
	}

	return nil
}

// for conditional updates via pointers because IDK how to SQL
type LinkUpdate struct {
	Short *string `json:"short,omitempty"`
	Url   *string `json:"url,omitempty"`
	Desc  *string `json:"desc,omitempty"`
}

func (update LinkUpdate) Validate() error {
	if update.Short != nil && *update.Short == "" {
		return errors.New("short link cannot be empty")
	}

	if update.Url == nil {
		return nil // happy path
	}

	if err := validateUrl(*update.Url); err != nil {
		return fmt.Errorf("full url must be a valid URL: %w", err)
	}

	return nil
}

// We could just check for sql.ErrNoRows, but this is a slightly
// more directly expressed requirement, in case we're a Mongo/Redis shop
// or something.
type StoreError interface {
	error
	IsNotFound() bool
}
