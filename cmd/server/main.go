package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	tunnelPort = ":8080"
	publicPort = ":9090"
)

type Tunnel struct {
	conn net.Conn
	mu   sync.RWMutex
}

var activeTunnel = &Tunnel{}

func main() {
	go startTunnelServer()
	startPublicServer()
}

func startTunnelServer() {
	listener, err := net.Listen("tcp", tunnelPort)
	if err != nil {
		log.Fatal("Failed to start tunnel server:", err)
	}
	defer listener.Close()

	log.Printf("🔌 Tunnel server listening on %s", tunnelPort)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Accept error:", err)
			continue
		}

		activeTunnel.mu.Lock()
		if activeTunnel.conn != nil {
			activeTunnel.conn.Close()
			log.Println("⚠️  Closed previous tunnel connection")
		}
		activeTunnel.conn = conn
		activeTunnel.mu.Unlock()

		log.Println("✅ Home server connected via tunnel")

		go func(c net.Conn) {
			buf := make([]byte, 1)
			for {
				c.SetReadDeadline(time.Now().Add(30 * time.Second))
				_, err := c.Read(buf)
				if err != nil {
					log.Println("🔌 Tunnel disconnected:", err)
					activeTunnel.mu.Lock()
					if activeTunnel.conn == c {
						activeTunnel.conn = nil
					}
					activeTunnel.mu.Unlock()
					c.Close()
					return
				}
			}
		}(conn)
	}
}

func startPublicServer() {
	http.HandleFunc("/", handlePublicRequest)
	http.HandleFunc("/health", handleHealth)

	log.Printf("🌐 Public API listening on %s", publicPort)
	log.Fatal(http.ListenAndServe(publicPort, nil))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	activeTunnel.mu.RLock()
	connected := activeTunnel.conn != nil
	activeTunnel.mu.RUnlock()

	if connected {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Tunnel: Connected\n")
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "Tunnel: Disconnected\n")
	}
}

func handlePublicRequest(w http.ResponseWriter, r *http.Request) {
	activeTunnel.mu.RLock()
	tunnel := activeTunnel.conn
	activeTunnel.mu.RUnlock()

	if tunnel == nil {
		http.Error(w, "Service temporarily unavailable - tunnel not connected", http.StatusServiceUnavailable)
		return
	}

	log.Printf("📨 %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer serverConn.Close()
		if err := r.Write(serverConn); err != nil {
			log.Println("Error writing request to tunnel:", err)
		}
	}()

	go func() {
		defer wg.Done()

		resp, err := http.ReadResponse(bufio.NewReader(clientConn), r)
		if err != nil {
			log.Println("Error reading response from tunnel:", err)
			return
		}
		defer resp.Body.Close()

		for k, v := range resp.Header {
			for _, val := range v {
				w.Header().Add(k, val)
			}
		}
		w.WriteHeader(resp.StatusCode)

		if _, err := io.Copy(w, resp.Body); err != nil {
			log.Println("Error copying response body:", err)
		}
	}()

	wg.Wait()
	log.Printf("✅ %s %s -> completed", r.Method, r.URL.Path)
}
