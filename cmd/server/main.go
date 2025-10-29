package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	http.HandleFunc("/tunnel", handleTunnel)
	fmt.Println("🚀 Tunnel server listening on :8080")
	http.ListenAndServe(":8080", nil)
}

func handleTunnel(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Upgrade error:", err)
		return
	}
	defer ws.Close()

	fmt.Println("🔌 Client connected")

	for {
		mt, message, err := ws.ReadMessage()
		if err != nil {
			fmt.Println("Client disconnected:", err)
			return
		}
		if mt != websocket.BinaryMessage {
			continue
		}

		fmt.Printf("Received %d bytes from client\n", len(message))

		// Echo back for now (we’ll later forward this to a TCP service)
		err = ws.WriteMessage(websocket.BinaryMessage, message)
		if err != nil {
			fmt.Println("Write error:", err)
			return
		}
	}
}
