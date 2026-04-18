package tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// DefaultServer is the public localtunnel.me instance.
const DefaultServer = "https://localtunnel.me"

// Tunnel proxies connections from a public URL to a local port using the
// localtunnel protocol (https://github.com/localtunnel/server).
type Tunnel struct {
	// URL is the public HTTPS URL assigned by the tunnel server.
	URL string

	localPort  int
	remoteHost string
	remotePort int
	maxConn    int
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

type assignment struct {
	ID           string `json:"id"`
	Port         int    `json:"port"`
	URL          string `json:"url"`
	MaxConnCount int    `json:"max_conn_count"`
}

// Start opens a tunnel from the default localtunnel.me server to localPort.
func Start(ctx context.Context, localPort int) (*Tunnel, error) {
	return StartWithServer(ctx, localPort, DefaultServer)
}

// StartWithServer opens a tunnel using a localtunnel-compatible server.
func StartWithServer(ctx context.Context, localPort int, serverURL string) (*Tunnel, error) {
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("parse server URL: %w", err)
	}

	info, err := requestTunnel(ctx, serverURL)
	if err != nil {
		return nil, err
	}

	if info.URL == "" || info.Port == 0 {
		return nil, fmt.Errorf("invalid tunnel assignment: missing URL or port")
	}

	tctx, cancel := context.WithCancel(ctx)

	t := &Tunnel{
		URL:        info.URL,
		localPort:  localPort,
		remoteHost: parsed.Hostname(),
		remotePort: info.Port,
		maxConn:    info.MaxConnCount,
		ctx:        tctx,
		cancel:     cancel,
	}

	if t.maxConn <= 0 {
		t.maxConn = 10
	}

	for i := 0; i < t.maxConn; i++ {
		t.wg.Add(1)
		go t.worker()
	}

	return t, nil
}

func requestTunnel(ctx context.Context, serverURL string) (*assignment, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL+"/?new", nil)
	if err != nil {
		return nil, fmt.Errorf("build tunnel request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request tunnel: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("tunnel server returned %d: %s", resp.StatusCode, string(body))
	}

	var info assignment
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode tunnel assignment: %w", err)
	}

	return &info, nil
}

func (t *Tunnel) worker() {
	defer t.wg.Done()
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			if err := t.proxy(); err != nil {
				// Brief pause before reconnecting on error.
				select {
				case <-t.ctx.Done():
					return
				case <-time.After(time.Second):
				}
			}
		}
	}
}

func (t *Tunnel) proxy() error {
	d := net.Dialer{Timeout: 10 * time.Second}
	remote, err := d.DialContext(t.ctx, "tcp", fmt.Sprintf("%s:%d", t.remoteHost, t.remotePort))
	if err != nil {
		return err
	}

	// Ensure the remote connection is closed when the context is cancelled,
	// which unblocks the blocking ReadFull below.
	proxyDone := make(chan struct{})
	defer close(proxyDone)
	go func() {
		select {
		case <-t.ctx.Done():
			remote.Close()
		case <-proxyDone:
		}
	}()
	defer remote.Close()

	// Block until the tunnel server forwards a request to this connection.
	header := make([]byte, 1)
	if _, err := io.ReadFull(remote, header); err != nil {
		return err
	}

	local, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", t.localPort), 5*time.Second)
	if err != nil {
		return err
	}
	defer local.Close()

	// Forward the byte we already consumed.
	if _, err := local.Write(header); err != nil {
		return err
	}

	// Bidirectional copy.
	errc := make(chan error, 2)
	go func() { _, err := io.Copy(local, remote); errc <- err }()
	go func() { _, err := io.Copy(remote, local); errc <- err }()

	select {
	case <-errc:
	case <-t.ctx.Done():
	}

	return nil
}

// Close shuts down the tunnel and waits for all proxy workers to exit.
func (t *Tunnel) Close() {
	t.cancel()
	t.wg.Wait()
}
