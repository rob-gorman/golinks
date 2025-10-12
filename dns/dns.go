package dns

import (
	"context"

	"github.com/rob-gorman/golinks/api/store"
	"github.com/rob-gorman/golinks/internal/logger"
)

type Resolver interface {
	Resolve(context.Context, string) (string, error)
}

type recordStore interface {
	GetLink(context.Context, string) (store.GoLink, error)
}

func DefaultResolver(store recordStore, log logger.Logger) Resolver {
	return &defaultResolver{store: store}
}

type defaultResolver struct {
	store recordStore
	cache cache
	log   logger.Logger
}

func (r *defaultResolver) Resolve(ctx context.Context, short string) (string, error) {
	if url, ok := r.cache.Get(ctx, short); ok {
		return url, nil
	}

	link, err := r.store.GetLink(ctx, short)
	if err != nil {
		return "", err
	}

	if err := r.cache.Set(ctx, short, link.Url); err != nil {
		r.log.Error("failed to set cache", "error", err)
	}

	return link.Url, nil
}

type cache struct{} // TODO

func (c *cache) Get(ctx context.Context, short string) (string, bool) {
	return "", false
}

func (c *cache) Set(ctx context.Context, short string, url string) error {
	return nil
}
