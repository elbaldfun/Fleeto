package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Agent represents a connected client
type Agent struct {
	ID       string
	Token    string
	Conn     *websocket.Conn
	LastPing time.Time
	mu       sync.RWMutex
}

// Server represents the main server instance
type Server struct {
	agents   map[string]*Agent
	upgrader websocket.Upgrader
	mu       sync.RWMutex
}

func NewServer() *Server {
	return &Server{
		agents: make(map[string]*Agent),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// TODO: Implement proper origin checking
				return true
			},
		},
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Websocket upgrade failed: %v", err)
		return
	}

	// Handle agent registration
	agent := &Agent{
		Conn:     conn,
		LastPing: time.Now(),
	}

	// TODO: Implement authentication and generate Agent ID/Token

	s.mu.Lock()
	s.agents[agent.ID] = agent
	s.mu.Unlock()

	go s.handleAgentConnection(agent)
}

func (s *Server) handleAgentConnection(agent *Agent) {
	defer func() {
		agent.Conn.Close()
		s.mu.Lock()
		delete(s.agents, agent.ID)
		s.mu.Unlock()
	}()

	for {
		messageType, _, err := agent.Conn.ReadMessage()
		if err != nil {
			log.Printf("Read error: %v", err)
			return
		}

		// Handle different message types
		switch messageType {
		case websocket.PingMessage:
			agent.mu.Lock()
			agent.LastPing = time.Now()
			agent.mu.Unlock()

			if err := agent.Conn.WriteMessage(websocket.PongMessage, nil); err != nil {
				log.Printf("Failed to send pong: %v", err)
				return
			}
		}

		// TODO: Implement message handling logic
	}
}

func main() {
	server := NewServer()

	// Start heartbeat checker
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for range ticker.C {
			server.checkHeartbeats()
		}
	}()

	// Configure TLS
	tlsConfig := &tls.Config{
		// TODO: Configure TLS certificates
		MinVersion: tls.VersionTLS12,
	}

	// Setup HTTP server
	httpServer := &http.Server{
		Addr:      ":8443",
		TLSConfig: tlsConfig,
	}

	http.HandleFunc("/ws", server.handleWebSocket)

	log.Printf("Server starting on :8443")
	if err := httpServer.ListenAndServeTLS("server.crt", "server.key"); err != nil {
		log.Fatal("ListenAndServeTLS: ", err)
	}
}

func (s *Server) checkHeartbeats() {
	threshold := time.Now().Add(-time.Minute)

	s.mu.Lock()
	defer s.mu.Unlock()

	for id, agent := range s.agents {
		agent.mu.RLock()
		if agent.LastPing.Before(threshold) {
			log.Printf("Agent %s timed out", id)
			agent.Conn.Close()
			delete(s.agents, id)
		}
		agent.mu.RUnlock()
	}
}
