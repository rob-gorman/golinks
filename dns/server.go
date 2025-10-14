package dns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
	"github.com/rob-gorman/golinks/internal/logger"
	"golang.org/x/sync/errgroup"
)

const (
	_recordTTL = 300 // 5 minutes
)

// server implements a DNS server that can resolve custom go-links
// and forward all other requests to an upstream server.
type server struct {
	resolver  resolver // resolves our golinks to full URLs
	dnsServer *dns.Server
}

// creates and manages the lifecycle of a new DNS server. This blocks.
func RunServer(ctx context.Context,
	redirector net.TCPAddr,
	prefix string,
	upstreamDns string,
	port int,
	log logger.Logger,
) error {

	log.Info("starting DNS server", "prefix", prefix, "upstreamDns", upstreamDns)
	resolver := newResolver(redirector, prefix, upstreamDns, log)

	server := &server{
		resolver: resolver,
	}

	g, ctx := errgroup.WithContext(ctx)
	ready := new(atomic.Bool) // for race condition between serve and shutdown

	g.Go(func() error {
		if err := server.serve(port, ready); err != nil {
			log.Error("failed to start DNS server", "error", err)
			return err
		}
		return nil
	})

	g.Go(func() error {
		<-ctx.Done()
		timeout := 0
		for !ready.Load() {
			if timeout > 100 {
				return errors.New("timed out waiting for DNS server to start")
			}
			time.Sleep(50 * time.Millisecond)
			timeout++
		}
		sdCtx, sdCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer sdCancel()
		return server.shutdown(sdCtx)
	})

	return g.Wait()
}

// starts the DNS server, listening on the optional address (":dns" if empty). This blocks.
func (s *server) serve(port int, ready *atomic.Bool) error {
	addr := fmt.Sprintf(":%d", port)
	s.dnsServer = &dns.Server{Addr: addr, Net: "udp"}
	dns.HandleFunc(".", s.resolver.resolve)
	ready.Store(true)
	return s.dnsServer.ListenAndServe()
}

// Shutdown gracefully shuts down the DNS server. This blocks until closed or cancelled.
func (s *server) shutdown(ctx context.Context) error {
	errch := make(chan error, 1)
	go func() {
		errch <- s.dnsServer.Shutdown()
	}()

	select {
	case <-ctx.Done():
		return errors.New("timed out waiting for DNS server to shutdown")
	case err := <-errch:
		return err
	}
}
