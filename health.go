package main

import (
	"log"
	"net/http"
	"os"
)

// HealthServer implements a *http.Server as server
type HealthServer struct {
	server *http.Server
}

// NewHealthServer returns a HealthServer
func NewHealthServer() *HealthServer {
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Pong"))
	})

	port := "8080"
	envHealthPort := os.Getenv("SW_HEALTH_PORT")
	if len(envHealthPort) != 0 {
		port = envHealthPort
	}
	Addr := ":" + port
	log.Printf("SET health server port to %v", Addr)

	return &HealthServer{
		server: &http.Server{
			Addr:    Addr,
			Handler: mux,
		},
	}
}

func (s *HealthServer) start() {
	log.Println("Starting health server")
	err := s.server.ListenAndServe()
	if err != nil {
		log.Fatalf("Failed to serve health server on %v - %v", s.server.Addr, err)
	}
}

func (s *HealthServer) close() {
	err := s.server.Close()
	if err != nil {
		log.Printf("Error closing down health server - %v", err)
	} else {
		log.Println("Successfully closed health server")
	}
}
