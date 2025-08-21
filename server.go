package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Server represents the HTTP server
type Server struct {
	server *http.Server
	logger *slog.Logger
}

// NewServer creates a new HTTP server
func NewServer(port string, mux *http.ServeMux, logger *slog.Logger) *Server {

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{
		server: server,
		logger: logger,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Create a channel to listen for errors coming from the listener
	serverErrors := make(chan error, 1)

	// Start the server in a goroutine
	go func() {
		s.logger.Info("Starting HTTP server", "port", s.server.Addr)
		serverErrors <- s.server.ListenAndServe()
	}()

	// Create a channel to listen for an interrupt or terminate signal from the OS
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Blocking select waiting for either a server error or a shutdown signal
	select {
	case err := <-serverErrors:
		return fmt.Errorf("error starting server: %w", err)

	case sig := <-shutdown:
		s.logger.Info("Shutting down server", "signal", sig)

		// Give outstanding requests a deadline for completion
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Gracefully shutdown the server
		if err := s.server.Shutdown(ctx); err != nil {
			s.logger.Error("Could not stop server gracefully", "error", err)
			if err := s.server.Close(); err != nil {
				return fmt.Errorf("could not force close server: %w", err)
			}
		}
	}

	return nil
}

// Stop gracefully shuts down the server
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	return s.server.Shutdown(ctx)
}
