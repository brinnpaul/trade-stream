package main

import (
	"log/slog"
	"net/http"
	"trade-stream/controllers"
	"trade-stream/data"
)

// SetupRoutes configures all the HTTP routes for the application
func SetupRoutes(priceManager data.PriceManager, logger *slog.Logger) *http.ServeMux {
	mux := http.NewServeMux()

	// Create ticker controller
	tickerController := controllers.NewPriceController(priceManager, logger)
	tickerViewController := controllers.NewPriceWsViewController()

	// Http Ticker price routes
	mux.HandleFunc("/api/prices", tickerController.GetLiveTickerPrices)

	// WebSocket Ticker price routes
	mux.HandleFunc("/ws/prices", tickerController.WebSocketPrices)

	// Serve Ticker View Page
	mux.HandleFunc("/prices", tickerViewController.ServiceTickerPage)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"trade-stream"}`))
	})

	return mux
}
