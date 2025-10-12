package dns

import (
	"context"
	"net"
	"testing"
	"time"

	miekgdns "github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

func TestCustomResolution_ARecord(t *testing.T) {
	t.Parallel()

	// 1) Create server with fake resolver mapping short -> URL with IPv4 host.
	resolver := &fakeResolver{resolve: func(ctx context.Context, short string) (string, error) {
		return "http://127.0.0.1", nil
	}}

	prefix := miekgdns.Fqdn("go")
	srv := &server{
		resolver: resolver,
		// upstream won't be used for matching prefix
		upstream: "127.0.0.1:65535",
		prefix:   prefix,
	}

	addr, shutdown := startDNSServer(t, srv)
	t.Cleanup(shutdown)

	// 2) Query using miekg/dns client
	c := &miekgdns.Client{Net: "udp", Timeout: 2 * time.Second}
	m := new(miekgdns.Msg)
	m.SetQuestion("foo."+prefix, miekgdns.TypeA)

	r, _, err := c.Exchange(m, addr)
	require.NoError(t, err)
	require.NotNil(t, r)
	require.Equal(t, miekgdns.RcodeSuccess, r.Rcode)
	require.NotEmpty(t, r.Answer)

	// Expect at least one A record for 127.0.0.1
	found := false
	for _, rr := range r.Answer {
		if a, ok := rr.(*miekgdns.A); ok && a.A.Equal(net.IPv4(127, 0, 0, 1)) {
			found = true
			break
		}
	}
	require.True(t, found, "expected 127.0.0.1 A record")
}

func TestForwarding_UsesUpstream(t *testing.T) {
	t.Parallel()

	// Upstream light server responds with 9.9.9.9 for example.com.
	upstreamAddr, upstreamShutdown := startLightUpstreamA(t, map[string]net.IP{
		"example.com.": net.IPv4(9, 9, 9, 9),
	})
	defer upstreamShutdown()

	srv := &server{
		resolver: &fakeResolver{resolve: func(ctx context.Context, short string) (string, error) { return "", context.Canceled }},
		upstream: upstreamAddr,
		prefix:   miekgdns.Fqdn("go."),
	}

	addr, shutdown := startDNSServer(t, srv)
	t.Cleanup(shutdown)

	c := &miekgdns.Client{Net: "udp", Timeout: 2 * time.Second}
	m := new(miekgdns.Msg)
	m.SetQuestion("example.com.", miekgdns.TypeA)

	r, _, err := c.Exchange(m, addr)
	require.NoError(t, err)
	require.Equal(t, miekgdns.RcodeSuccess, r.Rcode)

	found := false
	for _, rr := range r.Answer {
		if a, ok := rr.(*miekgdns.A); ok && a.A.Equal(net.IPv4(9, 9, 9, 9)) {
			found = true
			break
		}
	}
	require.True(t, found, "expected upstream A record 9.9.9.9")
}

func TestCustomResolution_NonA_ReturnsNameError(t *testing.T) {
	t.Parallel()

	srv := &server{
		resolver: &fakeResolver{resolve: func(ctx context.Context, short string) (string, error) { return "http://127.0.0.1", nil }},
		upstream: "127.0.0.1:65535",
		prefix:   miekgdns.Fqdn("go."),
	}
	addr, shutdown := startDNSServer(t, srv)
	t.Cleanup(shutdown)

	c := &miekgdns.Client{Net: "udp", Timeout: 2 * time.Second}
	m := new(miekgdns.Msg)
	m.SetQuestion("foo."+srv.prefix, miekgdns.TypeAAAA)

	r, _, err := c.Exchange(m, addr)
	require.NoError(t, err)
	require.Equal(t, miekgdns.RcodeNameError, r.Rcode)
}

func TestCustomResolution_NotFound_ReturnsNameError(t *testing.T) {
	t.Parallel()

	resolver := &fakeResolver{resolve: func(ctx context.Context, short string) (string, error) {
		return "", context.DeadlineExceeded
	}}

	srv := &server{resolver: resolver, upstream: "127.0.0.1:65535", prefix: miekgdns.Fqdn("go.")}
	addr, shutdown := startDNSServer(t, srv)
	t.Cleanup(shutdown)

	c := &miekgdns.Client{Net: "udp", Timeout: 2 * time.Second}
	m := new(miekgdns.Msg)
	m.SetQuestion("missing."+srv.prefix, miekgdns.TypeA)

	r, _, err := c.Exchange(m, addr)
	require.NoError(t, err)
	require.Equal(t, miekgdns.RcodeNameError, r.Rcode)
}


// fakeResolver implements Resolver for tests
type fakeResolver struct {
	resolve func(ctx context.Context, short string) (string, error)
}

func (f *fakeResolver) Resolve(ctx context.Context, short string) (string, error) {
	return f.resolve(ctx, short)
}

// startDNSServer starts our server bound to an ephemeral UDP port and returns address and shutdown.
func startDNSServer(t *testing.T, srv *server) (addr string, shutdown func()) {
	t.Helper()

	// Bind to an ephemeral port first so we can discover the port number.
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)

	mux := miekgdns.NewServeMux()
	mux.HandleFunc(srv.prefix, srv.handleCustom)
	mux.HandleFunc(".", srv.handleForward)

	// Wire up server with our PacketConn and mux, then activate.
	srv.dnsServer = &miekgdns.Server{PacketConn: pc, Handler: mux}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.dnsServer.ActivateAndServe()
	}()

	// Give it a moment to start
	deadline := time.Now().Add(2 * time.Second)
	for {
		// If ActivateAndServe exited early, fail fast
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("dns server failed to start: %v", err)
			}
		default:
		}

		// Try a non-blocking write to ensure the socket is live
		if pc.LocalAddr() != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("dns server did not start in time")
		}
		time.Sleep(10 * time.Millisecond)
	}

	la := pc.LocalAddr().String()

	shutdown = func() {
		// Shutdown will close PacketConn
		_ = srv.Shutdown()
	}

	return la, shutdown
}

// startUpstreamStub starts a minimal DNS server responding with the provided handler.
func startUpstreamStub(t *testing.T, handler func(w miekgdns.ResponseWriter, r *miekgdns.Msg)) (addr string, shutdown func()) {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)

	mux := miekgdns.NewServeMux()
	mux.HandleFunc(".", handler)

	s := &miekgdns.Server{PacketConn: pc, Handler: mux}
	go func() { _ = s.ActivateAndServe() }()

	return pc.LocalAddr().String(), func() { _ = s.Shutdown() }
}

// startLightUpstreamA starts a minimal upstream DNS server that serves static A records.
// answers map should contain fully-qualified domain names (with trailing dot) to IPv4 addresses.
func startLightUpstreamA(t *testing.T, answers map[string]net.IP) (addr string, shutdown func()) {
	t.Helper()
	return startUpstreamStub(t, func(w miekgdns.ResponseWriter, r *miekgdns.Msg) {
		m := new(miekgdns.Msg)
		m.SetReply(r)
		m.Authoritative = true
		for _, q := range r.Question {
			if q.Qtype != miekgdns.TypeA {
				continue
			}
			if ip, ok := answers[q.Name]; ok {
				rr := &miekgdns.A{Hdr: miekgdns.RR_Header{Name: q.Name, Rrtype: miekgdns.TypeA, Class: miekgdns.ClassINET, Ttl: 10}, A: ip.To4()}
				m.Answer = append(m.Answer, rr)
			}
		}
		_ = w.WriteMsg(m)
	})
}
