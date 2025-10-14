package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"
	_ "modernc.org/sqlite"

	"github.com/rob-gorman/golinks/api/store"
	"github.com/rob-gorman/golinks/dns"
	"github.com/rob-gorman/golinks/internal/logger"
	"github.com/rob-gorman/golinks/redirector"
)

func main() {
	ctx := context.Background()
	if err := run(ctx, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "[DNS SERVER] error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var (
		fs = flag.NewFlagSet(args[0], flag.ExitOnError)

		sqlite   = fs.String("sqlite", ":memory:", "sqlite db file to use")
		prefix   = fs.String("prefix", "go", "prefix to use for custom links")
		upstream = fs.String("upstream", "1.1.1.1:53", "upstream dns server to use")
		ipaddr   = fs.String("ip", "127.0.0.0", "network IP address of this server")
		dnsport  = fs.Int("dns", 53, "port to listen on for DNS")
		httpport = fs.Int("http", 80, "port to listen on for HTTP")
	)

	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	log := logger.Default()
	log.Info(fmt.Sprintf("started with args: %#+v", args))

	ip := net.ParseIP(*ipaddr)
	if ip == nil {
		return fmt.Errorf("invalid IP address: %s", *ipaddr)
	}
	httpaddr := net.TCPAddr{IP: ip, Port: *httpport}

	db, err := sql.Open("sqlite", *sqlite)
	if err != nil {
		return err
	}
	defer db.Close()

	linkdb, err := store.NewSqlStore(db)
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return redirector.Run(ctx, linkdb, log, redirector.WithPort(*httpport))
	})

	g.Go(func() error {
		return dns.RunServer(ctx, httpaddr, *prefix, *upstream, *dnsport, log)
	})

	return g.Wait()
}
