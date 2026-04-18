package tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockTunnelServer simulates a localtunnel-compatible server.
// It accepts tunnel assignment requests and forwards connections between the
// public side and the pooled client connections.
type mockTunnelServer struct {
	// proxyListener is where the tunnel client opens pooled connections.
	proxyListener net.Listener
	// httpServer is the assignment endpoint.
	httpServer *httptest.Server

	mu       sync.Mutex
	pool     []net.Conn
	poolCond *sync.Cond
}

func newMockTunnelServer(t *testing.T) *mockTunnelServer {
	t.Helper()

	proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen proxy: %v", err)
	}

	m := &mockTunnelServer{
		proxyListener: proxyListener,
	}
	m.poolCond = sync.NewCond(&m.mu)

	proxyPort := proxyListener.Addr().(*net.TCPAddr).Port

	m.httpServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := assignment{
			ID:           "test-tunnel",
			Port:         proxyPort,
			URL:          fmt.Sprintf("http://127.0.0.1:%d", proxyPort),
			MaxConnCount: 2,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}))

	// Accept pooled connections from the tunnel client.
	go func() {
		for {
			conn, err := proxyListener.Accept()
			if err != nil {
				return
			}
			m.mu.Lock()
			m.pool = append(m.pool, conn)
			m.poolCond.Broadcast()
			m.mu.Unlock()
		}
	}()

	t.Cleanup(func() {
		m.httpServer.Close()
		proxyListener.Close()
	})

	return m
}

// getPooledConn blocks until a pooled connection is available and returns it.
func (m *mockTunnelServer) getPooledConn(timeout time.Duration) (net.Conn, bool) {
	deadline := time.After(timeout)
	done := make(chan struct{})

	var conn net.Conn

	go func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		for len(m.pool) == 0 {
			m.poolCond.Wait()
		}
		conn = m.pool[0]
		m.pool = m.pool[1:]
		close(done)
	}()

	select {
	case <-done:
		return conn, true
	case <-deadline:
		return nil, false
	}
}

func TestTunnelProxiesHTTPRequest(t *testing.T) {
	// Start a local HTTP server (the "application" behind the tunnel).
	localListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen local: %v", err)
	}
	localPort := localListener.Addr().(*net.TCPAddr).Port

	localServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "hello from local")
		}),
	}
	go localServer.Serve(localListener)
	defer localServer.Close()

	// Set up mock tunnel server.
	mock := newMockTunnelServer(t)

	// Start the tunnel client.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tun, err := StartWithServer(ctx, localPort, mock.httpServer.URL)
	if err != nil {
		t.Fatalf("StartWithServer: %v", err)
	}
	defer tun.Close()

	if tun.URL == "" {
		t.Fatal("tunnel URL is empty")
	}

	// Wait for a pooled connection to appear.
	pooledConn, ok := mock.getPooledConn(5 * time.Second)
	if !ok {
		t.Fatal("no pooled connection from tunnel client")
	}

	// Simulate a request arriving at the tunnel: write an HTTP request to the
	// pooled connection and read back the response.
	reqStr := "GET /callback?code=abc&state=xyz HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n"
	if _, err := pooledConn.Write([]byte(reqStr)); err != nil {
		t.Fatalf("write to pooled conn: %v", err)
	}

	pooledConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	respBytes, err := io.ReadAll(pooledConn)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	resp := string(respBytes)
	if !strings.Contains(resp, "hello from local") {
		t.Fatalf("unexpected response: %s", resp)
	}
}

func TestTunnelCloseStopsWorkers(t *testing.T) {
	// Local listener that we never actually serve, just need the port.
	localListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	localPort := localListener.Addr().(*net.TCPAddr).Port
	defer localListener.Close()

	mock := newMockTunnelServer(t)

	ctx := context.Background()
	tun, err := StartWithServer(ctx, localPort, mock.httpServer.URL)
	if err != nil {
		t.Fatalf("StartWithServer: %v", err)
	}

	// Close should return promptly without hanging.
	done := make(chan struct{})
	go func() {
		tun.Close()
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(5 * time.Second):
		t.Fatal("Close() did not return within timeout")
	}
}

func TestRequestTunnelBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "overloaded", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, err := requestTunnel(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Fatalf("error should mention status code: %v", err)
	}
}

func TestRequestTunnelBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not json")
	}))
	defer srv.Close()

	_, err := requestTunnel(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

func TestStartWithServerRejectsMissingURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(assignment{Port: 1234})
	}))
	defer srv.Close()

	_, err := StartWithServer(context.Background(), 8080, srv.URL)
	if err == nil {
		t.Fatal("expected error for missing URL in assignment")
	}
	if !strings.Contains(err.Error(), "missing URL or port") {
		t.Fatalf("unexpected error: %v", err)
	}
}
