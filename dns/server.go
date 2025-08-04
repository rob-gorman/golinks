package dns

import (
	"context"
	"net"
	"net/url"
	"strings"
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
	resolver  Resolver // resolves our golinks to full URLs
	upstream  string   // upstream DNS server
	prefix    string   // prefix for internally resolved requests
	dnsServer *dns.Server
}

// Newserver creates a new DNS server.
func Runserver(ctx context.Context,
	resolver Resolver,
	upstreamDns string,
	prefix string,
	log logger.Logger,
) error {

	server := &server{
		resolver: resolver,
		upstream: upstreamDns,
		prefix:   dns.Fqdn(prefix),
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := server.ListenAndServe(":53"); err != nil {
			log.Error("failed to start DNS server", "error", err)
			return err
		}
		return nil
	})

	g.Go(func() error {
		<-ctx.Done()
		return server.Shutdown()
	})

	return g.Wait()
}

// starts the DNS server, listening on the optional address (":dns" if empty). This blocks.
func (s *server) ListenAndServe(addr string) error {
	s.dnsServer = &dns.Server{Addr: addr, Net: "udp"}
	dns.HandleFunc(s.prefix, s.handleCustom)
	dns.HandleFunc(".", s.handleForward)
	return s.dnsServer.ListenAndServe()
}

// Shutdown gracefully shuts down the DNS server.
func (s *server) Shutdown() error {
	if s.dnsServer != nil {
		return s.dnsServer.Shutdown()
	}
	return nil
}

// handleCustom resolves go-links for the configured suffix.
func (s *server) handleCustom(w dns.ResponseWriter, r *dns.Msg) {
	rCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second) // not clear
	defer cancel()

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true

	if len(r.Question) == 0 {
		handleFailed(w, r, dns.RcodeNameError)
		return
	}

	q := r.Question[0]
	if q.Qtype != dns.TypeA {
		handleFailed(w, r, dns.RcodeNameError)
		return
	}

	shortLink := strings.TrimPrefix(q.Name, s.prefix)
	fullURL, err := s.resolver.Resolve(rCtx, shortLink)
	if err != nil {
		handleFailed(w, r, dns.RcodeNameError)
		return
	}

	parsedURL, err := url.Parse(fullURL)
	if err != nil || parsedURL.Host == "" {
		handleFailed(w, r, dns.RcodeNameError)
		return
	}

	ips, err := net.LookupIP(parsedURL.Host)
	if err != nil {
		handleFailed(w, r, dns.RcodeServerFailure)
		return
	}

	for _, ip := range ips {
		if ip.To4() != nil {
			rr := &dns.A{
				Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: _recordTTL},
				A:   ip.To4(),
			}
			m.Answer = append(m.Answer, rr)
		}
	}

	if len(m.Answer) == 0 {
		m.SetRcode(r, dns.RcodeNameError)
	}

	w.WriteMsg(m)
}

// handleForward forwards all other DNS requests to the upstream server.
func (s *server) handleForward(w dns.ResponseWriter, r *dns.Msg) {
	resp, err := dns.Exchange(r, s.upstream)
	if err != nil {
		handleFailed(w, r, dns.RcodeServerFailure)
		return
	}
	w.WriteMsg(resp)
}

func handleFailed(w dns.ResponseWriter, r *dns.Msg, err int) {
	m := new(dns.Msg)
	m.SetRcode(r, err)
	w.WriteMsg(m)
}
