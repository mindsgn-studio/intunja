package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	remoteAddr = flag.String("remote", "http://localhost:8080", "Remote tunnel server address")
	localAddr  = flag.String("local", "http://localhost:3000", "Local API server address")
	reconnect  = flag.Duration("reconnect", 5*time.Second, "Reconnect delay")
	keepalive  = flag.Duration("keepalive", 10*time.Second, "Keep-alive interval")
	timeout    = flag.Duration("timeout", 30*time.Second, "Request timeout")
)

type TunnelClient struct {
	remoteAddr string
	localAddr  string
	conn       net.Conn
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

func main() {
	flag.Parse()

	log.Println("🏠 Home Server Tunnel Client")
	log.Printf("📡 Remote Tunnel: %s", *remoteAddr)
	log.Printf("🔗 Local API: %s", *localAddr)
	log.Println("Press Ctrl+C to stop")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := &TunnelClient{
		remoteAddr: *remoteAddr,
		localAddr:  *localAddr,
		ctx:        ctx,
		cancel:     cancel,
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\n🛑 Shutting down gracefully...")
		cancel()
	}()

	// Start tunnel with auto-reconnect
	client.Run()
}

func (tc *TunnelClient) Run() {
	for {
		select {
		case <-tc.ctx.Done():
			log.Println("✅ Tunnel client stopped")
			tc.wg.Wait()
			return
		default:
			if err := tc.connect(); err != nil {
				log.Printf("❌ Tunnel error: %v", err)
				log.Printf("🔄 Reconnecting in %v...", *reconnect)

				select {
				case <-tc.ctx.Done():
					return
				case <-time.After(*reconnect):
					continue
				}
			}
		}
	}
}

func (tc *TunnelClient) connect() error {
	log.Printf("🔌 Connecting to tunnel server at %s...", tc.remoteAddr)

	// Connect to remote tunnel server
	conn, err := net.DialTimeout("tcp", tc.remoteAddr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	tc.mu.Lock()
	tc.conn = conn
	tc.mu.Unlock()

	log.Println("✅ Tunnel established!")

	// Start keep-alive
	tc.wg.Add(1)
	go tc.keepAlive()

	// Handle incoming requests
	if err := tc.handleRequests(); err != nil {
		conn.Close()
		return err
	}

	return nil
}

func (tc *TunnelClient) keepAlive() {
	defer tc.wg.Done()

	ticker := time.NewTicker(*keepalive)
	defer ticker.Stop()

	for {
		select {
		case <-tc.ctx.Done():
			return
		case <-ticker.C:
			tc.mu.RLock()
			conn := tc.conn
			tc.mu.RUnlock()

			if conn == nil {
				return
			}

			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := conn.Write([]byte{0}); err != nil {
				log.Println("⚠️  Keep-alive failed:", err)
				conn.Close()
				return
			}
		}
	}
}

func (tc *TunnelClient) handleRequests() error {
	tc.mu.RLock()
	conn := tc.conn
	tc.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("no active connection")
	}

	reader := bufio.NewReader(conn)

	for {
		select {
		case <-tc.ctx.Done():
			return fmt.Errorf("context cancelled")
		default:
		}

		// Set read deadline
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// Read HTTP request
		req, err := http.ReadRequest(reader)
		if err != nil {
			// Check if it's a keep-alive byte
			if err == io.EOF {
				return fmt.Errorf("tunnel closed by remote server")
			}

			// Try to read as raw byte (keep-alive)
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}

			return fmt.Errorf("failed to read request: %w", err)
		}

		// Handle request in separate goroutine
		tc.wg.Add(1)
		go tc.handleRequest(req)
	}
}

func (tc *TunnelClient) handleRequest(req *http.Request) {
	defer tc.wg.Done()

	// Build local URL
	localURL := tc.localAddr + req.URL.String()

	log.Printf("📨 %s %s from tunnel", req.Method, req.URL.Path)

	// Create new request to local API
	ctx, cancel := context.WithTimeout(tc.ctx, *timeout)
	defer cancel()

	localReq, err := http.NewRequestWithContext(ctx, req.Method, localURL, req.Body)
	if err != nil {
		log.Printf("❌ Failed to create local request: %v", err)
		tc.sendErrorResponse(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	// Copy headers
	localReq.Header = req.Header.Clone()

	// Add/update forwarding headers
	if req.RemoteAddr != "" {
		localReq.Header.Set("X-Forwarded-For", req.RemoteAddr)
	}
	localReq.Header.Set("X-Forwarded-Proto", "http")

	// Forward to local API
	client := &http.Client{
		Timeout: *timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}

	resp, err := client.Do(localReq)
	if err != nil {
		log.Printf("❌ Local API error: %v", err)
		tc.sendErrorResponse(http.StatusBadGateway, "Bad Gateway - Local API Error")
		return
	}
	defer resp.Body.Close()

	// Send response back through tunnel
	if err := tc.sendResponse(resp); err != nil {
		log.Printf("❌ Failed to send response through tunnel: %v", err)
		return
	}

	log.Printf("✅ %s %s → %d (%s)", req.Method, req.URL.Path, resp.StatusCode, resp.Status)
}

func (tc *TunnelClient) sendResponse(resp *http.Response) error {
	tc.mu.RLock()
	conn := tc.conn
	tc.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("no active connection")
	}

	// Set write deadline
	conn.SetWriteDeadline(time.Now().Add(30 * time.Second))

	// Write the full HTTP response
	var buf bytes.Buffer
	if err := resp.Write(&buf); err != nil {
		return fmt.Errorf("failed to serialize response: %w", err)
	}

	if _, err := conn.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}

	return nil
}

func (tc *TunnelClient) sendErrorResponse(statusCode int, message string) {
	tc.mu.RLock()
	conn := tc.conn
	tc.mu.RUnlock()

	if conn == nil {
		return
	}

	resp := &http.Response{
		StatusCode:    statusCode,
		Status:        fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewBufferString(message)),
		ContentLength: int64(len(message)),
	}

	resp.Header.Set("Content-Type", "text/plain; charset=utf-8")
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(message)))

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	resp.Write(conn)
}
