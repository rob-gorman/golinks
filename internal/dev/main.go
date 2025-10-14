package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/rob-gorman/golinks/api/store"
	_ "modernc.org/sqlite"
)

var _seedLinks = []store.GoLink{
	{
		Short: "yt",
		Url:   "https://youtube.com",
		Desc:  "A test link",
	},
	{
		Short: "gh",
		Url:   "https://github.com",
		Desc:  "A test link 2",
	},
	{
		Short: "htmx",
		Url:   "https://htmx.org/docs/#installing",
		Desc:  "htmx docs",
	},
}

func main() {
	ctx := context.Background()
	if err := run(ctx, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	outfile := flag.String("outfile", "build/golinks", "sqlite db file to create")
	flag.Parse()

	db, err := sql.Open("sqlite", *outfile)
	if err != nil {
		return err
	}
	defer db.Close()

	linkdb, err := store.NewSqlStore(db)
	if err != nil {
		return err
	}

	for _, link := range _seedLinks {
		_, err := linkdb.CreateLink(ctx, link)
		if err != nil && !strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return err
		}
	}

	if err := contents(ctx, linkdb); err != nil {
		return err
	}

	return nil
}

func contents(ctx context.Context, linkdb store.Store) error {
	links, err := linkdb.ListLinks(ctx)
	if err != nil {
		return err
	}
	for _, link := range links {
		fmt.Println(link.Short, link.Url, link.Desc)
	}
	return nil
}
