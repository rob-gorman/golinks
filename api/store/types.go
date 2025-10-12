package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
)

type Store interface {
	GetLink(context.Context, string) (GoLink, error)
	CreateLink(context.Context, GoLink) (GoLink, error) // maybe return just ID
	UpdateLink(context.Context, LinkUpdate, string) error
	DeleteLink(context.Context, string) error
	ListLinks(context.Context) ([]GoLink, error)
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

// IsNotFound standardizes not-found checks for callers without exposing SQL details.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, sql.ErrNoRows)
}
