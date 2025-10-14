package dns

import (
	"fmt"
	"net"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/miekg/dns"
	"github.com/rob-gorman/golinks/internal/logger"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	ctx := t.Context()
	log := logger.Default()

	dnsport := 8082
	httpport := 8083
	addr := fmt.Sprintf("127.0.0.1:%d", dnsport)

	httpaddr := net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: httpport,
	}

	go func() {
		err := RunServer(ctx, httpaddr, "go", "1.1.1.1:53", dnsport, log)
		t.Cleanup(func() {
			require.NoError(t, err)
		})
	}()

	time.Sleep(1 * time.Second)
	t.Run("resolve upstream", func(t *testing.T) {
		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn("google.com"), dns.TypeA)
		resp, err := dns.Exchange(m, addr)
		require.NoError(t, err)
		require.Equal(t, dns.RcodeSuccess, resp.Rcode)
	})

	t.Run("resolve link", func(t *testing.T) {
		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn("go"), dns.TypeA)
		resp, err := dns.Exchange(m, addr)
		require.NoError(t, err)
		require.Equal(t, dns.RcodeSuccess, resp.Rcode)
		require.Equal(t, httpaddr.IP.To4(), resp.Answer[0].(*dns.A).A.To4())
	})
}
