package dns

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
	"github.com/rob-gorman/golinks/internal/logger"
)

type resolver struct {
	resolution net.TCPAddr // the IP address to which to resolve the shortlink prefix
	prefix     string // identifier for our custom links
	upstream   string // upstream DNS server to forward requests
	log        logger.Logger
}

func newResolver(
	httpRedirector net.TCPAddr,
	prefix, upstream string,
	log logger.Logger,
) resolver {
	return resolver{
		resolution: httpRedirector,
		prefix:     prefix,
		upstream:   upstream,
		log:        log,
	}
}

func (rslv resolver) resolve(w dns.ResponseWriter, r *dns.Msg) {
	rslv.log.Debug("resolving request", "request", r)
	if len(r.Question) == 0 {
		handleFailed(w, r, dns.RcodeNameError)
		return
	}

	q := r.Question[0]
	if q.Qtype != dns.TypeA {
		handleFailed(w, r, dns.RcodeNameError)
		return
	}

	switch {
	case strings.HasPrefix(q.Name, rslv.prefix):
		rslv.handleLink(w, r)
	default:
		rslv.handleUpstream(w, r)
	}
}

// convenience wrapper around [forward]
func (rslv resolver) handleUpstream(w dns.ResponseWriter, r *dns.Msg) {
	resp, err := rslv.forward(r)
	if err != nil {
		handleFailed(w, r, dns.RcodeServerFailure)
		return
	}
	w.WriteMsg(resp)
}

// handleCustom resolves go-links for the configured suffix.
func (rslv resolver) handleLink(w dns.ResponseWriter, r *dns.Msg) {
	q := r.Question[0]
	resp := new(dns.Msg)
	resp.SetReply(r)
	resp.Answer = append(resp.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: _recordTTL},
		A:   rslv.resolution.IP.To4(),
	})

	w.WriteMsg(resp)
}

// we ask upstream for A records, so we need to append our custom link to the answer
func (rslv resolver) rewriteAnswer(q dns.Question, resp *dns.Msg) *dns.Msg {
	for _, ans := range resp.Answer {
		if a, ok := ans.(*dns.A); ok {
			rr := &dns.A{
				Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: _recordTTL},
				A:   a.A.To4(),
			}
			resp.Answer = append(resp.Answer, rr)
		}
	}
	return resp
}

func (rslv resolver) forward(r *dns.Msg) (*dns.Msg, error) {
	resp, err := dns.Exchange(r, rslv.upstream)
	if err != nil {
		rslv.log.Error("failed to forward request", "error", err, "domain", r.Question[0].Name)
		return nil, fmt.Errorf("failed to forward request: %w", err)
	}
	return resp, nil
}

func handleFailed(w dns.ResponseWriter, r *dns.Msg, err int) {
	m := new(dns.Msg)
	m.SetRcode(r, err)
	w.WriteMsg(m)
}

type cache struct{} // TODO

func (c *cache) Get(ctx context.Context, short string) (string, bool) {
	return "", false
}

func (c *cache) Set(ctx context.Context, short string, url string) error {
	return nil
}
