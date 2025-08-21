package controllers

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"trade-stream/data"
	"trade-stream/models"
)

func TestPriceController_GetLiveTickerPrices(t *testing.T) {
	controller := NewPriceController(data.NewInMemoryPriceManager(slog.Default()), slog.Default())

	// Create a test request
	req, err := http.NewRequest("GET", "/api/prices", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Call the handler
	controller.GetLiveTickerPrices(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check the content type
	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("handler returned wrong content type: got %v want %v", contentType, "application/json")
	}
}

func TestPriceController_GetLiveTickerPrices_WithSymbolsParam(t *testing.T) {
	controller := NewPriceController(data.NewInMemoryPriceManager(slog.Default()), slog.Default())

	// Create a test request with symbol parameter
	req, err := http.NewRequest("GET", "/api/prices?symbol=BTCUSDT", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Call the handler
	controller.GetLiveTickerPrices(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check the content type
	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("handler returned wrong content type: got %v want %v", contentType, "application/json")
	}
}

func TestPriceController_GetLiveTickerPricesForSymbols_MultipleRequests(t *testing.T) {
	// Create a mock price manager for testing
	mockPriceManager := &MockPriceManager{}
	controller := NewPriceController(mockPriceManager, slog.Default())

	// Test case 1: Valid symbols
	req, err := http.NewRequest("GET", "/api/prices?symbols=BTCUSDT,ETHUSDT", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	controller.GetLiveTickerPrices(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Test case 2: Empty symbols parameter
	req2, err := http.NewRequest("GET", "/api/prices/symbols?symbols=", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr2 := httptest.NewRecorder()
	controller.GetLiveTickerPrices(rr2, req2)

	if status := rr2.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}

	// Test case 3: Invalid symbols (only whitespace)
	req3, err := http.NewRequest("GET", "/api/prices/symbols?symbols=  ,  ", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr3 := httptest.NewRecorder()
	controller.GetLiveTickerPrices(rr3, req3)

	if status := rr3.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}

// MockPriceManager is a mock implementation for testing
type MockPriceManager struct{}

func (m *MockPriceManager) Subscribe() chan models.Prices {
	return make(chan models.Prices)
}

func (m *MockPriceManager) Unsubscribe(ch chan models.Prices) {}

func (m *MockPriceManager) Start() {}

func (m *MockPriceManager) Shutdown() {}

func (m *MockPriceManager) GetPrices() models.Prices {
	// Return mock Data for testing
	prices := models.EmptyPrices()
	prices.UpdatePriceTicker(models.PriceUpdate{Symbol: "BTCUSDT", Price: "50000.00"})
	prices.UpdatePriceTicker(models.PriceUpdate{Symbol: "ETHUSDT", Price: "3000.00"})
	return *prices
}

func (m *MockPriceManager) GetDoneChannel() chan struct{} {
	return make(chan struct{})
}

func (m *MockPriceManager) GetPriceUpdateChannel() chan models.PriceUpdate {
	return make(chan models.PriceUpdate)
}
