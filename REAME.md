# Complete Guide to Tunnel Services

## Table of Contents
1. [Why and Who Needs Tunnel Services](#why-and-who-needs-tunnel-services)
2. [Fundamentals of Tunnels](#fundamentals-of-tunnels)
3. [Server Tunnel Architecture](#server-tunnel-architecture)
4. [Client Tunnel Architecture](#client-tunnel-architecture)
5. [Practical Implementation](#practical-implementation)

---

## Why and Who Needs Tunnel Services

### The Problem: NAT and Firewalls

Most home internet connections face these challenges:

1. **Dynamic IP Addresses**: Your home IP changes frequently
2. **NAT (Network Address Translation)**: Multiple devices share one public IP
3. **Firewall Restrictions**: ISPs block incoming connections
4. **CGNAT (Carrier-Grade NAT)**: Multiple customers share the same public IP
5. **Port Blocking**: ISPs block common ports (80, 443, 22)

**Result:** You can't directly access services running on your home network from the internet.

### Who Needs Tunnel Services?

#### 1. **Developers and Hobbyists**
```
Scenario: Running development APIs at home
Problem: Need to test webhooks from external services (Stripe, GitHub, etc.)
Solution: Tunnel exposes local API to receive webhooks
```

#### 2. **Self-Hosters**
```
Scenario: Running home server with personal services
- Personal blog/website
- File storage (Nextcloud)
- Media server (Plex, Jellyfin)
- Home automation (Home Assistant)
Problem: Can't access from outside without exposing home network
Solution: Secure tunnel through VPS
```

#### 3. **Small Businesses**
```
Scenario: Office server with CRM/inventory system
Problem: Remote workers need access, but office has dynamic IP
Solution: Stable tunnel through cloud VPS
```

#### 4. **IoT and Remote Monitoring**
```
Scenario: Raspberry Pi monitoring sensors at home
Problem: Need to access data remotely
Solution: Lightweight tunnel to cloud server
```

#### 5. **Cost-Conscious Deployments**
```
Scenario: Need public API but cloud hosting is expensive
Home Setup: High-RAM server with 32GB RAM
Cloud VPS: Minimal $5/month server for tunnel only
Benefit: Save hundreds on hosting costs
```

### Real-World Use Cases

| Use Case | Without Tunnel | With Tunnel |
|----------|----------------|-------------|
| **Webhook Testing** | Can't receive webhooks | Webhooks work instantly |
| **Demo Apps** | Can't share with clients | Share via public URL |
| **Home Lab Access** | Only works at home | Access from anywhere |
| **Collaborative Dev** | Complex VPN setup | Simple URL sharing |
| **API Development** | Deploy to cloud constantly | Test locally, expose globally |

### Alternatives and When to Use Tunnels

**Don't use tunnels when:**
- You need enterprise-grade uptime (use cloud hosting)
- Your home internet is unreliable
- You're serving high-traffic production apps
- You need low latency globally (use CDN)

**Do use tunnels when:**
- Development and testing
- Personal projects
- Low to medium traffic services
- Learning and experimentation
- Cost is a constraint

---

## Fundamentals of Tunnels

### What is a Tunnel?

A tunnel is a **persistent connection** that allows traffic to flow in the reverse direction of the connection establishment.

```
Normal Web Request:
Client → Server (Client initiates, Server responds)

Tunnel:
Home Server → VPS (Home initiates and maintains connection)
Client → VPS → Home Server (VPS forwards traffic through existing connection)
```

### Core Concepts

#### 1. **Connection Inversion**

```
Traditional Model:
┌────────┐         ┌──────────┐
│ Client │ ──────→ │  Server  │
└────────┘         └──────────┘
   Initiates       Accepts

Tunnel Model:
┌────────────┐         ┌─────┐         ┌──────────┐
│ Home Server│ ←─────→ │ VPS │ ←────── │  Client  │
└────────────┘         └─────┘         └──────────┘
   Initiates         Broker          Initiates
   (Outbound OK)    (Public IP)      (Normal Request)
```

**Key Insight:** Home server makes outbound connection (allowed by firewalls), then VPS uses this connection to forward inbound requests.

#### 2. **Persistent Connection**

```go
// Home server maintains long-lived connection
for {
    conn = dialTunnelServer()
    defer conn.Close()
    
    // Keep connection open and wait for requests
    for request := range readRequests(conn) {
        response := forwardToLocalAPI(request)
        sendResponse(conn, response)
    }
    
    // Auto-reconnect if disconnected
    time.Sleep(reconnectDelay)
}
```

The connection stays open indefinitely, acting as a "pipe" for bidirectional communication.

#### 3. **Request-Response Flow**

```
Step-by-step:

1. Home Server connects to VPS
   Home ──(TCP)──→ VPS
   
2. Connection established and maintained
   Home ←──────→ VPS
   
3. External client makes request
   Client ──(HTTP)──→ VPS:9090
   
4. VPS forwards through tunnel
   VPS ──(HTTP via tunnel)──→ Home
   
5. Home processes and responds
   Home ──(HTTP via tunnel)──→ VPS
   
6. VPS forwards response to client
   VPS ──(HTTP)──→ Client
```

### Types of Tunnels

#### 1. **TCP Tunnel** (Our Implementation)
```
Pros:
✓ Works with any TCP protocol
✓ Full HTTP/HTTPS support
✓ Binary data support
✓ Low overhead

Cons:
✗ No built-in encryption (use SSH or TLS)
✗ Must implement protocol handling
```

#### 2. **SSH Tunnel**
```bash
ssh -R 9090:localhost:3000 user@vps

Pros:
✓ Built-in encryption
✓ Authentication included
✓ No custom code needed

Cons:
✗ Requires SSH access
✗ SSH overhead
✗ One tunnel per command
```

#### 3. **WebSocket Tunnel**
```
Pros:
✓ Works through HTTP proxies
✓ Bidirectional
✓ Web-friendly

Cons:
✗ Higher overhead
✗ More complex framing
```

#### 4. **HTTP/2 or gRPC Tunnel**
```
Pros:
✓ Multiplexing (multiple streams)
✓ Modern protocol features
✓ Efficient

Cons:
✗ Complex implementation
✗ More dependencies
```

### Security Considerations

#### 1. **Encryption Layer**

```
Without Encryption:
Client → VPS → Tunnel → Home
         ↑              ↑
      Plain text    Plain text

With TLS:
Client → VPS → Tunnel → Home
      HTTPS    TLS?     HTTP
```

**Best Practice:** Add TLS to tunnel connection or use SSH tunnel.

#### 2. **Authentication**

```go
// Add token-based authentication
type TunnelHandshake struct {
    Token     string
    ClientID  string
    Timestamp int64
}

func authenticateTunnel(conn net.Conn) error {
    var handshake TunnelHandshake
    if err := json.NewDecoder(conn).Decode(&handshake); err != nil {
        return err
    }
    
    if !validateToken(handshake.Token) {
        return errors.New("invalid token")
    }
    
    return nil
}
```

#### 3. **Rate Limiting**

Prevent abuse by limiting connections per IP or requests per minute.

### Network Topology

```
Internet
    │
    ├─── Client A (anywhere)
    │
    ├─── Client B (anywhere)
    │
    ├─── Client C (anywhere)
    │
    ▼
┌─────────────────┐
│   VPS Server    │ ← Public IP: 123.45.67.89
│  Port 8080      │ ← Tunnel port (for home server)
│  Port 9090      │ ← Public API port (for clients)
└─────────────────┘
         ▲
         │ Persistent TCP Connection (outbound from home)
         │
    ┌────┴─────┐
    │  Router  │ ← NAT/Firewall (allows outbound)
    │   NAT    │
    └────┬─────┘
         │
    ┌────▼─────┐
    │   Home   │ ← Private IP: 192.168.1.100
    │  Server  │ ← Runs tunnel client
    └──────────┘
         │
    ┌────▼─────┐
    │ Local API│ ← Localhost: 127.0.0.1:3000
    └──────────┘
```

---

## Server Tunnel Architecture

### Overview

The server tunnel runs on your VPS with a public IP and acts as a **broker** between public clients and your home server.

### Responsibilities

1. **Accept tunnel connections** from home server
2. **Accept public requests** from clients
3. **Bridge traffic** between clients and home server
4. **Maintain connection health**
5. **Handle disconnections gracefully**

### Architecture Diagram

```
┌──────────────────────────────────────────────────┐
│              Tunnel Server (VPS)                 │
│                                                  │
│  ┌────────────────┐      ┌────────────────┐    │
│  │ Tunnel Listener│      │ Public Listener│    │
│  │   Port 8080    │      │   Port 9090    │    │
│  └───────┬────────┘      └────────┬───────┘    │
│          │                         │             │
│          │                         │             │
│          ▼                         ▼             │
│  ┌───────────────────────────────────────┐     │
│  │        Connection Manager              │     │
│  │  - Track active tunnel connection      │     │
│  │  - Queue/route requests                │     │
│  │  - Health monitoring                   │     │
│  └───────────────────────────────────────┘     │
│                      │                           │
│                      ▼                           │
│  ┌───────────────────────────────────────┐     │
│  │         Request Forwarder              │     │
│  │  - Pipe public requests to tunnel      │     │
│  │  - Pipe tunnel responses to clients    │     │
│  └───────────────────────────────────────┘     │
└──────────────────────────────────────────────────┘
```

### Key Components

#### 1. **Tunnel Listener**

```go
func startTunnelServer() {
    listener, err := net.Listen("tcp", ":8080")
    if err != nil {
        log.Fatal("Failed to start tunnel server:", err)
    }
    
    for {
        conn, err := listener.Accept()
        if err != nil {
            log.Println("Accept error:", err)
            continue
        }
        
        // Store the tunnel connection
        handleTunnelConnection(conn)
    }
}
```

**Purpose:** Accept incoming connections from home servers.

**Design Decisions:**
- Only one active tunnel per home server (closes old connection if new one arrives)
- Non-blocking accept loop
- Connection validation (could add authentication here)

#### 2. **Public API Listener**

```go
func startPublicServer() {
    http.HandleFunc("/", handlePublicRequest)
    log.Fatal(http.ListenAndServe(":9090", nil))
}

func handlePublicRequest(w http.ResponseWriter, r *http.Request) {
    // Check if tunnel is connected
    if activeTunnel == nil {
        http.Error(w, "Service Unavailable", 503)
        return
    }
    
    // Forward request through tunnel
    forwardRequest(w, r, activeTunnel)
}
```

**Purpose:** Accept HTTP requests from public clients.

**Design Decisions:**
- Standard HTTP server (easy to add TLS)
- Health check endpoint
- Returns 503 if tunnel disconnected

#### 3. **Connection Manager**

```go
type Tunnel struct {
    conn net.Conn
    mu   sync.RWMutex
}

var activeTunnel = &Tunnel{}

func handleTunnelConnection(conn net.Conn) {
    activeTunnel.mu.Lock()
    
    // Close old connection
    if activeTunnel.conn != nil {
        activeTunnel.conn.Close()
    }
    
    // Store new connection
    activeTunnel.conn = conn
    activeTunnel.mu.Unlock()
    
    // Monitor connection health
    monitorConnection(conn)
}
```

**Thread Safety:** Uses mutex to protect shared connection state.

**Connection Strategy:** Single active tunnel (could be extended to support multiple home servers with routing).

#### 4. **Request Forwarder**

```go
func forwardRequest(w http.ResponseWriter, r *http.Request, tunnel net.Conn) {
    // Create pipe for bidirectional communication
    clientConn, serverConn := net.Pipe()
    
    var wg sync.WaitGroup
    wg.Add(2)
    
    // Goroutine 1: Write request to tunnel
    go func() {
        defer wg.Done()
        defer serverConn.Close()
        r.Write(serverConn) // Serialize HTTP request
    }()
    
    // Goroutine 2: Read response from tunnel and send to client
    go func() {
        defer wg.Done()
        resp, _ := http.ReadResponse(bufio.NewReader(clientConn), r)
        
        // Copy headers
        for k, v := range resp.Header {
            w.Header()[k] = v
        }
        w.WriteHeader(resp.StatusCode)
        
        // Copy body
        io.Copy(w, resp.Body)
    }()
    
    wg.Wait()
}
```

**Key Technique:** Uses `net.Pipe()` to create in-memory bidirectional pipe.

**Flow:**
1. HTTP request serialized and sent to tunnel
2. Response read from tunnel
3. Response forwarded to HTTP client

### Concurrency Model

```
Multiple Clients → Single VPS → Single Tunnel → Home Server

Thread Safety:
- Each public request handled in separate goroutine
- Tunnel connection protected by mutex
- Concurrent reads/writes on same TCP connection are safe
```

### Error Handling

```go
// Handle tunnel disconnection
func monitorConnection(conn net.Conn) {
    buf := make([]byte, 1)
    for {
        conn.SetReadDeadline(time.Now().Add(30 * time.Second))
        _, err := conn.Read(buf)
        if err != nil {
            log.Println("Tunnel disconnected:", err)
            
            activeTunnel.mu.Lock()
            if activeTunnel.conn == conn {
                activeTunnel.conn = nil
            }
            activeTunnel.mu.Unlock()
            
            conn.Close()
            return
        }
    }
}
```

**Strategies:**
1. **Read deadline:** Detect stale connections
2. **Keep-alive messages:** Maintain connection
3. **Graceful degradation:** Return 503 when tunnel down

### Scalability Considerations

#### Single Home Server
```
Current implementation: 1 VPS ↔ 1 Home Server
Limitation: ~10,000 concurrent connections (depending on resources)
```

#### Multiple Home Servers
```go
// Map of home servers
type TunnelManager struct {
    tunnels map[string]*Tunnel // clientID -> connection
    mu      sync.RWMutex
}

func (tm *TunnelManager) Route(subdomain string) *Tunnel {
    tm.mu.RLock()
    defer tm.mu.RUnlock()
    return tm.tunnels[subdomain]
}
```

**Routing strategies:**
- Subdomain-based: `alice.tunnel.com` → Alice's home server
- Path-based: `/alice/*` → Alice's home server
- Header-based: `X-Tunnel-ID: alice` → Alice's home server

---

## Client Tunnel Architecture

### Overview

The client tunnel runs on your home server and maintains a persistent connection to the VPS, forwarding requests to your local API.

### Responsibilities

1. **Establish connection** to remote tunnel server
2. **Maintain connection** with keep-alive
3. **Listen for requests** from tunnel
4. **Forward requests** to local API
5. **Send responses** back through tunnel
6. **Auto-reconnect** on disconnection

### Architecture Diagram

```
┌─────────────────────────────────────────────────┐
│          Tunnel Client (Home Server)            │
│                                                 │
│  ┌────────────────────────────────────┐        │
│  │      Connection Manager             │        │
│  │  - Dial remote server               │        │
│  │  - Auto-reconnect logic             │        │
│  │  - Health monitoring                │        │
│  └────────────┬───────────────────────┘        │
│               │                                  │
│               ▼                                  │
│  ┌────────────────────────────────────┐        │
│  │       Keep-Alive Manager            │        │
│  │  - Send periodic heartbeats         │        │
│  │  - Detect stale connections         │        │
│  └────────────────────────────────────┘        │
│               │                                  │
│               ▼                                  │
│  ┌────────────────────────────────────┐        │
│  │      Request Handler                │        │
│  │  - Parse HTTP requests              │        │
│  │  - Forward to local API             │        │
│  │  - Handle responses                 │        │
│  └────────────┬───────────────────────┘        │
│               │                                  │
│               ▼                                  │
│  ┌────────────────────────────────────┐        │
│  │      Local API Client               │        │
│  │  - HTTP client to localhost         │        │
│  │  - Timeout handling                 │        │
│  │  - Error management                 │        │
│  └────────────────────────────────────┘        │
└─────────────────────────────────────────────────┘
              │
              ▼
     ┌─────────────────┐
     │   Local API     │
     │  localhost:3000 │
     └─────────────────┘
```

### Key Components

#### 1. **Connection Manager**

```go
func (tc *TunnelClient) Run() {
    for {
        select {
        case <-tc.ctx.Done():
            return // Graceful shutdown
        default:
            if err := tc.connect(); err != nil {
                log.Printf("Error: %v", err)
                time.Sleep(reconnectDelay)
                continue // Auto-reconnect
            }
        }
    }
}

func (tc *TunnelClient) connect() error {
    conn, err := net.DialTimeout("tcp", tc.remoteAddr, 10*time.Second)
    if err != nil {
        return fmt.Errorf("connection failed: %w", err)
    }
    
    tc.conn = conn
    
    // Start keep-alive
    go tc.keepAlive()
    
    // Handle requests
    return tc.handleRequests()
}
```

**Design Pattern:** Infinite retry loop with exponential backoff.

**Key Features:**
- Context-based cancellation for graceful shutdown
- Connection timeout prevents hanging
- Error propagation triggers reconnection

#### 2. **Keep-Alive Manager**

```go
func (tc *TunnelClient) keepAlive() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-tc.ctx.Done():
            return
        case <-ticker.C:
            tc.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
            if _, err := tc.conn.Write([]byte{0}); err != nil {
                log.Println("Keep-alive failed:", err)
                tc.conn.Close()
                return
            }
        }
    }
}
```

**Purpose:** Prevent NAT timeout and detect broken connections.

**Why needed:**
- NAT devices drop idle connections (typically after 60-300 seconds)
- Keep-alive ensures connection stays active
- Early detection of broken connections

**Byte sent:** Single null byte `{0}` - minimal overhead.

#### 3. **Request Handler**

```go
func (tc *TunnelClient) handleRequests() error {
    reader := bufio.NewReader(tc.conn)
    
    for {
        // Set read deadline
        tc.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
        
        // Read HTTP request from tunnel
        req, err := http.ReadRequest(reader)
        if err != nil {
            if err == io.EOF {
                return fmt.Errorf("tunnel closed")
            }
            return fmt.Errorf("read error: %w", err)
        }
        
        // Handle in separate goroutine
        go tc.handleRequest(req)
    }
}
```

**Concurrency:** Each request processed in separate goroutine.

**Benefits:**
- Multiple concurrent requests supported
- Long-running requests don't block others
- Better throughput

**Read Deadline:** Prevents infinite blocking on slow/stale connections.

#### 4. **Local API Forwarder**

```go
func (tc *TunnelClient) handleRequest(req *http.Request) {
    // Build local URL
    localURL := tc.localAddr + req.URL.String()
    
    // Create request with timeout
    ctx, cancel := context.WithTimeout(tc.ctx, 30*time.Second)
    defer cancel()
    
    localReq, _ := http.NewRequestWithContext(ctx, req.Method, localURL, req.Body)
    localReq.Header = req.Header.Clone()
    
    // Add forwarding headers
    localReq.Header.Set("X-Forwarded-For", req.RemoteAddr)
    localReq.Header.Set("X-Forwarded-Proto", "http")
    
    // Make request to local API
    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(localReq)
    if err != nil {
        tc.sendErrorResponse(http.StatusBadGateway, "Local API Error")
        return
    }
    defer resp.Body.Close()
    
    // Send response back through tunnel
    tc.sendResponse(resp)
}
```

**Key Points:**
- **Context timeout:** Prevents hanging on slow API
- **Header preservation:** Maintains original request headers
- **Forwarding headers:** Adds X-Forwarded-* for client info
- **Error handling:** Sends proper error responses

#### 5. **Response Sender**

```go
func (tc *TunnelClient) sendResponse(resp *http.Response) error {
    // Serialize entire HTTP response
    var buf bytes.Buffer
    if err := resp.Write(&buf); err != nil {
        return fmt.Errorf("failed to serialize: %w", err)
    }
    
    // Send through tunnel
    tc.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
    if _, err := tc.conn.Write(buf.Bytes()); err != nil {
        return fmt.Errorf("failed to send: %w", err)
    }
    
    return nil
}
```

**Response Format:** Complete HTTP response (status line, headers, body).

**Write Deadline:** Prevents blocking on slow/broken connections.

### State Management

```go
type TunnelClient struct {
    remoteAddr string
    localAddr  string
    conn       net.Conn
    mu         sync.RWMutex
    ctx        context.Context
    cancel     context.CancelFunc
    wg         sync.WaitGroup
}
```

**Thread Safety:**
- `mu`: Protects connection access
- `ctx`: Enables graceful shutdown
- `wg`: Tracks active goroutines

### Lifecycle Management

```
1. Start
   ↓
2. Connect to remote
   ↓
3. Start keep-alive (goroutine)
   ↓
4. Listen for requests
   ↓
5. Handle requests (goroutines)
   ↓
6. Connection error?
   ├─ Yes → Close → Sleep → Reconnect (goto 2)
   └─ No → Continue
   ↓
7. Shutdown signal?
   ├─ Yes → Cancel context → Wait for goroutines → Exit
   └─ No → Continue (goto 4)
```

### Error Recovery Strategies

#### 1. **Connection Failures**
```go
// Exponential backoff
var backoff = 5 * time.Second
for {
    if err := connect(); err != nil {
        log.Printf("Failed: %v, retrying in %v", err, backoff)
        time.Sleep(backoff)
        backoff = min(backoff*2, 60*time.Second)
    } else {
        backoff = 5 * time.Second // Reset on success
    }
}
```

#### 2. **Partial Failures**
```go
// Individual request failures don't kill connection
go func() {
    if err := handleRequest(req); err != nil {
        log.Printf("Request failed: %v", err)
        // Connection stays alive for other requests
    }
}()
```

#### 3. **Graceful Shutdown**
```go
// Wait for in-flight requests
tc.cancel()        // Signal shutdown
tc.wg.Wait()       // Wait for goroutines
tc.conn.Close()    // Close connection
```

### Performance Optimization

#### 1. **Connection Pooling** (for local API)
```go
var httpClient = &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
}
```

#### 2. **Buffer Management**
```go
// Reuse buffers
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 32*1024)
    },
}

buf := bufferPool.Get().([]byte)
defer bufferPool.Put(buf)
```

#### 3. **Concurrent Request Handling**
```go
// Semaphore to limit concurrent requests
var sem = make(chan struct{}, 100) // Max 100 concurrent

go func() {
    sem <- struct{}{}        // Acquire
    defer func() { <-sem }() // Release
    handleRequest(req)
}()
```

---

## Practical Implementation

### Complete Setup Walkthrough

#### Step 1: VPS Setup

```bash
# SSH into your VPS
ssh user@your-vps-ip

# Create directory
sudo mkdir -p /opt/tunnel-server
cd /opt/tunnel-server

# Create server.go with the tunnel server code
sudo nano server.go

# Initialize Go module
go mod init tunnel-server
go mod tidy

# Build
go build -o tunnel-server server.go

# Test run
./tunnel-server
```

#### Step 2: Systemd Service

```bash
# Create service file
sudo nano /etc/systemd/system/tunnel-server.service
```

```ini
[Unit]
Description=Tunnel Server
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/tunnel-server
ExecStart=/opt/tunnel-server/tunnel-server
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

```bash
# Start service
sudo systemctl daemon-reload
sudo systemctl enable tunnel-server
sudo systemctl start tunnel-server
sudo systemctl status tunnel-server
```

#### Step 3: Firewall Configuration

```bash
# UFW
sudo ufw allow 8080/tcp  # Tunnel port
sudo ufw allow 9090/tcp  # Public API

# firewalld
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --permanent --add-port=9090/tcp
sudo firewall-cmd --reload
```

#### Step 4: Home Server Setup

```bash
# On your home server
cd ~/
mkdir tunnel-client
cd tunnel-client

# Create client.go with the tunnel client code
nano client.go

# Edit configuration
# - Update remoteAddr to your VPS IP
# - Update localAddr to your local API

# Build
go mod init tunnel-client
go build -o tunnel-client client.go

# Test run
./tunnel-client -remote="YOUR_VPS_IP:8080" -local="http://localhost:3000"
```

#### Step 5: Create Local Test API

```bash
# Simple Python test server
python3 -m http.server 3000

# Or Node.js
npx http-server -p 3000

# Or Go
go run -m http.server -p 3000
```

#### Step 6: Testing

```bash
# From anywhere (not your home network)
curl http://YOUR_VPS_IP:9090/

# Should return your local API response
```

### Advanced Configuration

#### Adding TLS to Tunnel

```go
// Server side
cert, _ := tls.LoadX509KeyPair("server.crt", "server.key")
config := &tls.Config{Certificates: []tls.Certificate{cert}}

listener, _ := tls.Listen("tcp", ":8080", config)

// Client side
config := &tls.Config{InsecureSkipVerify: false}
conn, _ := tls.Dial("tcp", remoteAddr, config)
```

#### Adding Authentication

```go
// Simple token auth
const SECRET_TOKEN = "your-secret-token-here"

// Server validates on connection
func validateTunnel(conn net.Conn) error {
    var token string
    json.NewDecoder(conn).Decode(&token)
    
    if token != SECRET_TOKEN {
        return errors.New("invalid token")
    }
    return nil
}

// Client sends token
func connect() error {
    conn, _ := net.Dial("tcp", remoteAddr)
    json.NewEncoder(conn).Encode(SECRET_TOKEN)
    return nil
}
```

### Monitoring and Observability

#### Logging Best Practices

```go
// Structured logging
log.Printf("[%s] %s %s -> %d (%dms)",
    timestamp,
    req.Method,
    req.URL.Path,
    resp.StatusCode,
    duration.Milliseconds(),
)
```

#### Metrics Collection

```go
// Track metrics
type Metrics struct {
    TotalRequests   int64
    FailedRequests  int64
    AvgResponseTime float64
    ConnectionDrops int64
}

// Update metrics
atomic.AddInt64(&metrics.TotalRequests, 1)
```

#### Health Checks

```bash
# Check tunnel status
curl http://YOUR_VPS_IP:9090/health

# Response: "Tunnel: Connected" or "Tunnel: Disconnected"
```

### Production Checklist

- [ ] TLS enabled on tunnel connection
- [ ] Authentication implemented
- [ ] Rate limiting configured
- [ ] Monitoring/alerting set up
- [ ] Backup tunnel server configured
- [ ] Log rotation enabled
- [ ] Firewall rules tested
- [ ] Auto-restart on failure verified
- [ ] Documentation written
- [ ] Runbooks created for common issues

---

## Conclusion

Tunnel