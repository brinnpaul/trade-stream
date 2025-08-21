package controllers

import (
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"trade-stream/data"
	"trade-stream/models"

	"github.com/gorilla/websocket"
)

type PriceController struct {
	priceManager data.PriceManager
	upgrader     *websocket.Upgrader
	logger       *slog.Logger
}

// NewPriceController creates a new price controller
func NewPriceController(priceManager data.PriceManager, logger *slog.Logger) *PriceController {
	return &PriceController{
		priceManager: priceManager,
		upgrader: &websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// TODO check environment var
				return true
			},
		},
		logger: logger,
	}
}

// Http

// GetLiveTickerPrices handles GET request for all live ticker prices
func (tc *PriceController) GetLiveTickerPrices(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers for web clients
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight OPTIONS request
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Only allow GET requests
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	prices := tc.priceManager.GetPrices()

	// Set content type to JSON
	w.Header().Set("Content-Type", "application/json")

	// Return the ticker prices as JSON
	if err := json.NewEncoder(w).Encode(GetPriceResponse(prices.GetRawData())); err != nil {
		tc.logger.Error("Failed to encode price Data", "error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// GetLiveTickerPricesForSymbols handles GET request for all live ticker prices
func (tc *PriceController) GetLiveTickerPricesForSymbols(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers for web clients
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight OPTIONS request
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Only allow GET requests
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	prices := tc.priceManager.GetPrices()

	// Set content type to JSON
	w.Header().Set("Content-Type", "application/json")

	symbolsParam := r.URL.Query().Get("symbols")
	if symbolsParam == "" {
		http.Error(w, "Missing 'symbols' query parameter", http.StatusBadRequest)
		return
	}

	// Parse comma-separated symbols
	symbols := strings.Split(symbolsParam, ",")
	if len(symbols) == 0 {
		http.Error(w, "Invalid symbols parameter", http.StatusBadRequest)
		return
	}

	// Clean up symbols (trim whitespace and convert to uppercase)
	var cleanSymbols []string
	for _, symbol := range symbols {
		cleanSymbol := strings.TrimSpace(strings.ToUpper(symbol))
		if cleanSymbol != "" {
			cleanSymbols = append(cleanSymbols, cleanSymbol)
		}
	}

	if len(cleanSymbols) == 0 {
		http.Error(w, "No valid symbols provided", http.StatusBadRequest)
		return
	}

	// Filter prices for requested symbols
	filteredPrices := make(map[string]models.PriceTicker)
	for _, symbol := range cleanSymbols {
		if ticker := prices.GetPriceTicker(symbol); ticker.Symbol != "" {
			filteredPrices[symbol] = ticker
		}
	}

	// Return the ticker prices as JSON
	if err := json.NewEncoder(w).Encode(GetPriceResponse(filteredPrices)); err != nil {
		tc.logger.Error("Failed to encode price Data", "error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func GetPriceResponse(data map[string]models.PriceTicker) PriceResponse {
	responseData := []models.PriceTicker{}
	for _, ticker := range data {
		responseData = append(responseData, ticker)
	}
	return PriceResponse{Data: responseData}
}

// Websocket

// WebSocketPrices websocket connection that gives live prices via priceManager channel
func (tc *PriceController) WebSocketPrices(w http.ResponseWriter, r *http.Request) {
	conn, err := tc.upgrader.Upgrade(w, r, nil)
	if err != nil {
		tc.logger.Error("Failed to upgrade websocket connection", "error", err)
		return
	}

	defer func(conn *websocket.Conn) {
		connCloseErr := conn.Close()
		if connCloseErr != nil {
			log.Println(connCloseErr)
		}
	}(conn)

	tc.logger.Debug("New Connection established", "address", conn.RemoteAddr())

	priceChannel := tc.priceManager.Subscribe()
	defer tc.priceManager.Unsubscribe(priceChannel)

	connClosed := make(chan struct{})
	go func() {
		defer close(connClosed)

		// Read messages to detect disconnection
		for {
			_, _, readErr := conn.ReadMessage()
			if readErr != nil {
				if websocket.IsUnexpectedCloseError(readErr, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					tc.logger.Debug("WebSocket read error", "error", readErr)
				}
				return
			}
		}
	}()

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	initialPrices := tc.priceManager.GetPrices()
	tc.logger.Debug("Writing initial prices to websocket", "prices", initialPrices.GetRawData())
	if initialWriteErr := conn.WriteJSON(initialPrices.GetRawData()); initialWriteErr != nil {
		tc.logger.Error("Failed to send initial prices", "error", initialWriteErr)
		return
	}

	// Listen for price updates and forward to WebSocket client
	for {
		select {
		case <-tc.priceManager.GetDoneChannel():
			tc.logger.Debug("Received shutdown from price manager")
			return
		case prices := <-priceChannel:
			tc.logger.Debug("Forwarding updated prices to websocket")
			if priceWriteErr := conn.WriteJSON(prices.GetRawData()); priceWriteErr != nil {
				tc.logger.Error("Failed to send price update", "error", priceWriteErr)
				return
			}
		case <-pingTicker.C:
			if pingErr := conn.WriteMessage(websocket.PingMessage, nil); pingErr != nil {
				tc.logger.Error("Failed to send ping", "error", pingErr)
				return
			}
		case <-connClosed:
			tc.logger.Debug("WebSocket connection closed", "address", r.RemoteAddr)
			return
		case <-r.Context().Done():
			tc.logger.Debug("WebSocket client disconnected", "address", r.RemoteAddr)
			return
		}
	}
}
