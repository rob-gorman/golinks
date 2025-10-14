package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
	_ "modernc.org/sqlite"

	"github.com/rob-gorman/golinks/api/store"
	"github.com/stretchr/testify/require"
)

func seedSqliteFile(t *testing.T, dir string, links []store.GoLink) string {
	t.Helper()
	dbPath := filepath.Join(dir, "seed.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	st, err := store.NewSqlStore(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	for _, l := range links {
		_, err := st.CreateLink(t.Context(), store.GoLink{Short: l.Short, Url: l.Url, Desc: l.Short})
		if err != nil {
			t.Fatalf("seed link: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	return dbPath
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func freeUDPPort(t *testing.T) int {
	t.Helper()
	a, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolve udp: %v", err)
	}
	c, err := net.ListenUDP("udp", a)
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	port := c.LocalAddr().(*net.UDPAddr).Port
	c.Close()
	return port
}

func testClient(t *testing.T, dnsAddr string) *http.Client {
	t.Helper()
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			var d net.Dialer
			// honor the requested network ("udp" or "tcp") and point to our DNS server
			return d.DialContext(ctx, network, dnsAddr)
		},
	}
	d := &net.Dialer{Resolver: r}
	tr := &http.Transport{DialContext: d.DialContext}
	return &http.Client{
		Transport: tr,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 5 * time.Second,
	}
}

func TestRun(t *testing.T) {
	// not parallel: uses global dns.HandleFunc
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	dir := t.TempDir()

	links := []store.GoLink{
		{Short: "yt", Url: "https://youtube.com/any"},
		{Short: "gh", Url: "https://github.com/any"},
	}
	dbPath := seedSqliteFile(t, dir, links)

	httpPort := freeTCPPort(t)
	dnsPort := freeUDPPort(t)
	ip := "127.0.0.1"
	t.Logf("http port: %d, dns port: %d, ip: %s", httpPort, dnsPort, ip)

	args := []string{
		os.Args[0],
		"-sqlite", dbPath,
		"-prefix", "go",
		"-upstream", "1.1.1.1:53",
		"-ip", ip,
		"-dns", strconv.Itoa(dnsPort),
		"-http", strconv.Itoa(httpPort),
	}

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return run(ctx, args) })

	time.Sleep(300 * time.Millisecond)

	client := testClient(t, fmt.Sprintf("%s:%d", ip, dnsPort))

	for _, link := range links {
		t.Run(link.Short, func(t *testing.T) {
			// this URL looks unlike our use case, yes.
			// but we have this problem in browsers too. it requires a big hammer to
			// manage in either place. we're just testing the behavior of our server, and
			// dealing with client limitations here.
			url := fmt.Sprintf("http://go:%d/%s", httpPort, link.Short)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			require.NoError(t, err, "failed to create request")

			resp, err := client.Do(req)
			require.NoError(t, err, "failed to do request")
			require.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode, "unexpected status")
			require.Equal(t, link.Url, resp.Header.Get("Location"), "unexpected location")
		})
	}

	t.Run("not found", func(t *testing.T) {
		url := fmt.Sprintf("http://go:%d/not-found", httpPort)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		require.NoError(t, err, "failed to create request")
		resp, err := client.Do(req)
		require.NoError(t, err, "failed to do request")
		require.Equal(t, http.StatusNotFound, resp.StatusCode, "unexpected status")
	})

	t.Run("malformed", func(t *testing.T) {
		url := fmt.Sprintf("http://go:%d/too/many/segments", httpPort)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		require.NoError(t, err, "failed to create request")
		resp, err := client.Do(req)
		require.NoError(t, err, "failed to do request")
		require.Equal(t, http.StatusBadRequest, resp.StatusCode, "unexpected status")
	})

	cancel()
	require.NoError(t, g.Wait())
}
