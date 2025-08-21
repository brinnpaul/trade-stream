package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
	"trade-stream/client"
	"trade-stream/config"
	"trade-stream/models"
)

// Mock implementations for testing
type MockPriceClient struct {
	tickerPrices []client.TickerPrice
	shouldError  bool
}

func (m *MockPriceClient) GetAllLiveTickerPrices() ([]client.TickerPrice, error) {
	if m.shouldError {
		return nil, errors.New("mock error")
	}
	return m.tickerPrices, nil
}

type MockTradeStreamFactory struct {
	shouldError bool
	streams     []MockStream
}

type MockStream struct {
	symbols []string
	doneCh  chan struct{}
	stopCh  chan struct{}
}

func (m *MockTradeStreamFactory) GetNewStream(symbols []string, handler client.MultiStreamWsTradeHandler, errHandler client.ErrorHandler) (doneCh, stopCh chan struct{}, err error) {
	if m.shouldError {
		return nil, nil, errors.New("mock stream creation error")
	}

	stream := MockStream{
		symbols: symbols,
		doneCh:  make(chan struct{}),
		stopCh:  make(chan struct{}),
	}
	m.streams = append(m.streams, stream)

	// Simulate stream processing
	go func() {
		// Simulate some trade events
		event := &client.MultiStreamWsTradeEvent{
			Stream: "btcusdt@trade",
			Data: client.WsTradeEvent{
				Symbol: "BTCUSDT",
				Price:  "50000.00",
			},
		}
		handler(event)

		// Wait for stop signal
		<-stream.stopCh
		close(stream.doneCh)
	}()

	return stream.doneCh, stream.stopCh, nil
}

type MockPriceManager struct {
	priceUpdates chan models.PriceUpdate
	done         chan struct{}
	prices       models.Prices
}

func NewMockPriceManager() *MockPriceManager {
	return &MockPriceManager{
		priceUpdates: make(chan models.PriceUpdate, 100),
		done:         make(chan struct{}),
		prices:       *models.EmptyPrices(),
	}
}

func (m *MockPriceManager) Subscribe() chan models.Prices {
	return make(chan models.Prices)
}

func (m *MockPriceManager) Unsubscribe(ch chan models.Prices) {
	close(ch)
}

func (m *MockPriceManager) Start() {}

func (m *MockPriceManager) Shutdown() {
	close(m.done)
}

func (m *MockPriceManager) GetPrices() models.Prices {
	return m.prices
}

func (m *MockPriceManager) GetDoneChannel() chan struct{} {
	return m.done
}

func (m *MockPriceManager) GetPriceUpdateChannel() chan models.PriceUpdate {
	return m.priceUpdates
}

func TestNewBinancePriceLoader(t *testing.T) {
	cfg := &config.Config{}
	mockClient := &MockPriceClient{}
	mockFactory := &MockTradeStreamFactory{}
	mockManager := NewMockPriceManager()
	logger := slog.Default()

	loader := NewBinancePriceLoader(cfg, mockClient, mockFactory, mockManager, logger)

	if loader == nil {
		t.Fatal("Expected loader to be created, got nil")
	}
}

func TestBinancePriceLoader_LoadPrices(t *testing.T) {
	tests := []struct {
		name           string
		symbols        []string
		tickerPrices   []client.TickerPrice
		shouldError    bool
		expectedError  bool
		expectedEvents int
	}{
		{
			name:    "Successfully load prices for specific symbols",
			symbols: []string{"BTCUSDT", "ETHUSDT"},
			tickerPrices: []client.TickerPrice{
				{Symbol: "BTCUSDT", Price: "50000.00"},
				{Symbol: "ETHUSDT", Price: "3000.00"},
				{Symbol: "ETHBTC", Price: "3000.00"},
			},
			shouldError:    false,
			expectedError:  false,
			expectedEvents: 2, // Only BTCUSDT and ETHUSDT should be processed
		},
		{
			name:    "Client error should propagate",
			symbols: []string{"BTCUSDT"},
			tickerPrices: []client.TickerPrice{
				{Symbol: "BTCUSDT", Price: "50000.00"},
			},
			shouldError:   true,
			expectedError: true,
		},
		{
			name:           "Empty symbols list",
			symbols:        []string{},
			tickerPrices:   []client.TickerPrice{},
			shouldError:    false,
			expectedError:  false,
			expectedEvents: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPriceClient{
				tickerPrices: tt.tickerPrices,
				shouldError:  tt.shouldError,
			}
			mockManager := NewMockPriceManager()
			logger := slog.Default()

			loader := &BinancePriceLoader{
				config:        &config.Config{},
				binanceClient: mockClient,
				priceManager:  mockManager,
				logger:        logger,
			}

			err := loader.LoadPrices(tt.symbols)

			if tt.expectedError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectedError {
				// Check that the expected number of price updates were sent
				time.Sleep(10 * time.Millisecond) // Give time for goroutines to process
				select {
				case update := <-mockManager.priceUpdates:
					// Check if this symbol was in our requested list
					found := false
					for _, symbol := range tt.symbols {
						if update.Symbol == symbol {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Received update for unexpected symbol: %s", update.Symbol)
					}
				case <-time.After(100 * time.Millisecond):
					if tt.expectedEvents > 0 {
						t.Error("Expected price updates but received none")
					}
				}
			}
		})
	}
}

func TestBinancePriceLoader_LoadAllPrices(t *testing.T) {
	tests := []struct {
		name            string
		tickerPrices    []client.TickerPrice
		shouldError     bool
		expectedError   bool
		expectedSymbols int
	}{
		{
			name: "Successfully load all prices",
			tickerPrices: []client.TickerPrice{
				{Symbol: "BTCUSDT", Price: "50000.00"},
				{Symbol: "ETHUSDT", Price: "3000.00"},
				{Symbol: "ADAUSDT", Price: "0.50"},
			},
			shouldError:     false,
			expectedError:   false,
			expectedSymbols: 3,
		},
		{
			name:            "Client error should propagate",
			tickerPrices:    []client.TickerPrice{},
			shouldError:     true,
			expectedError:   true,
			expectedSymbols: 0,
		},
		{
			name:            "Empty response",
			tickerPrices:    []client.TickerPrice{},
			shouldError:     false,
			expectedError:   false,
			expectedSymbols: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPriceClient{
				tickerPrices: tt.tickerPrices,
				shouldError:  tt.shouldError,
			}
			mockManager := NewMockPriceManager()
			logger := slog.Default()

			loader := &BinancePriceLoader{
				config:        &config.Config{},
				binanceClient: mockClient,
				priceManager:  mockManager,
				logger:        logger,
			}

			symbols, err := loader.LoadAllPrices()

			if tt.expectedError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectedError {
				if len(symbols) != tt.expectedSymbols {
					t.Errorf("Expected %d symbols, got %d", tt.expectedSymbols, len(symbols))
				}

				// Check that price updates were sent for all symbols
				time.Sleep(10 * time.Millisecond) // Give time for goroutines to process
				for i := 0; i < tt.expectedSymbols; i++ {
					select {
					case update := <-mockManager.priceUpdates:
						// Verify the update has valid data
						if update.Symbol == "" {
							t.Error("Received update with empty symbol")
						}
						if update.Price == "" {
							t.Error("Received update with empty price")
						}
					case <-time.After(100 * time.Millisecond):
						t.Error("Expected price updates but received none")
						break
					}
				}
			}
		})
	}
}

func TestBinancePriceLoader_LoadPricesFromStream(t *testing.T) {
	tests := []struct {
		name             string
		symbols          []string
		symbolsPerStream int
		shouldError      bool
		expectedError    bool
		expectedStreams  int
	}{
		{
			name:             "Successfully create streams",
			symbols:          []string{"BTCUSDT", "ETHUSDT", "ADAUSDT"},
			symbolsPerStream: 2,
			shouldError:      false,
			expectedError:    false,
			expectedStreams:  2, // 3 symbols / 2 per stream = 2 streams
		},
		{
			name:             "Single stream for all symbols",
			symbols:          []string{"BTCUSDT", "ETHUSDT"},
			symbolsPerStream: 10,
			shouldError:      false,
			expectedError:    false,
			expectedStreams:  1,
		},
		{
			name:             "Factory error should propagate",
			symbols:          []string{"BTCUSDT"},
			symbolsPerStream: 1,
			shouldError:      true,
			expectedError:    true,
			expectedStreams:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFactory := &MockTradeStreamFactory{
				shouldError: tt.shouldError,
			}
			mockManager := NewMockPriceManager()
			logger := slog.Default()

			loader := &BinancePriceLoader{
				config:             &config.Config{},
				tradeStreamFactory: mockFactory,
				priceManager:       mockManager,
				logger:             logger,
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Run in goroutine since LoadPricesFromStream blocks
			errCh := make(chan error, 1)
			go func() {
				err := loader.LoadPricesFromStream(ctx, tt.symbols, tt.symbolsPerStream)
				errCh <- err
			}()

			// Give it time to process
			time.Sleep(100 * time.Millisecond)

			// Check if we got the expected number of streams
			if !tt.shouldError && len(mockFactory.streams) != tt.expectedStreams {
				t.Errorf("Expected %d streams, got %d", tt.expectedStreams, len(mockFactory.streams))
			}

			// call cancel to initiate shutdown
			cancel()

			// Wait for the function to complete
			select {
			case err := <-errCh:
				if tt.expectedError && err == nil {
					t.Error("Expected error but got none")
				}
				if !tt.expectedError && err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			case <-time.After(1 * time.Second):
				t.Error("Function did not complete within timeout")
			}
		})
	}
}

func TestBinancePriceLoader_TradeEventHandler(t *testing.T) {
	mockManager := NewMockPriceManager()
	logger := slog.Default()

	loader := &BinancePriceLoader{
		priceManager: mockManager,
		logger:       logger,
	}

	// Test trade event handling
	event := &client.MultiStreamWsTradeEvent{
		Stream: "btcusdt@trade",
		Data: client.WsTradeEvent{
			Symbol: "BTCUSDT",
			Price:  "50000.00",
		},
	}

	loader.TradeEventHandler(event)

	// Check that the price update was sent
	select {
	case update := <-mockManager.priceUpdates:
		if update.Symbol != "BTCUSDT" {
			t.Errorf("Expected symbol BTCUSDT, got %s", update.Symbol)
		}
		if update.Price != "50000.00" {
			t.Errorf("Expected price 50000.00, got %s", update.Price)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected price update but received none")
	}
}

func TestBinancePriceLoader_TradeEventHandler_Shutdown(t *testing.T) {
	mockManager := NewMockPriceManager()
	logger := slog.Default()

	loader := &BinancePriceLoader{
		priceManager: mockManager,
		logger:       logger,
	}

	// Shutdown the manager to simulate shutdown scenario
	mockManager.Shutdown()

	// Test trade event handling during shutdown
	event := &client.MultiStreamWsTradeEvent{
		Stream: "btcusdt@trade",
		Data: client.WsTradeEvent{
			Symbol: "BTCUSDT",
			Price:  "50000.00",
		},
	}

	// This should not block or cause issues
	loader.TradeEventHandler(event)

	// The main goal is to ensure no panic occurs
	// We don't need to verify the exact behavior of updates during shutdown
	// as this is implementation-dependent and may have race conditions
}

func TestCreateSymbolsSet(t *testing.T) {
	tests := []struct {
		name     string
		symbols  []string
		expected map[string]bool
	}{
		{
			name:    "Normal symbols",
			symbols: []string{"BTCUSDT", "ETHUSDT"},
			expected: map[string]bool{
				"BTCUSDT": true,
				"ETHUSDT": true,
			},
		},
		{
			name:     "Empty list",
			symbols:  []string{},
			expected: map[string]bool{},
		},
		{
			name:    "Duplicate symbols",
			symbols: []string{"BTCUSDT", "BTCUSDT", "ETHUSDT"},
			expected: map[string]bool{
				"BTCUSDT": true,
				"ETHUSDT": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createSymbolsSet(tt.symbols)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d symbols, got %d", len(tt.expected), len(result))
			}

			for symbol, expected := range tt.expected {
				if result[symbol] != expected {
					t.Errorf("Expected symbol %s to be %v, got %v", symbol, expected, result[symbol])
				}
			}
		})
	}
}

func TestGetSymbolFromStreamName(t *testing.T) {
	tests := []struct {
		name     string
		stream   string
		expected string
	}{
		{
			name:     "Normal stream name",
			stream:   "btcusdt@trade",
			expected: "BTCUSDT",
		},
		{
			name:     "Uppercase symbol",
			stream:   "ETHUSDT@trade",
			expected: "ETHUSDT",
		},
		{
			name:     "Mixed case symbol",
			stream:   "AdaUsdt@trade",
			expected: "ADAUSDT",
		},
		{
			name:     "Symbol with numbers",
			stream:   "1000floki@trade",
			expected: "1000FLOKI",
		},
		{
			name:     "Empty stream name",
			stream:   "@trade",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getSymbolFromStreamName(tt.stream)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestBinancePriceLoader_LoadPrices_TimeoutHandling(t *testing.T) {
	// Create a price manager with a blocked channel to test timeout
	mockManager := &MockPriceManager{
		priceUpdates: make(chan models.PriceUpdate, 1), // Buffer of 1
		done:         make(chan struct{}),
		prices:       *models.EmptyPrices(),
	}

	// Fill the buffer to block further writes
	mockManager.priceUpdates <- models.PriceUpdate{Symbol: "BLOCKED", Price: "0.00"}

	mockClient := &MockPriceClient{
		tickerPrices: []client.TickerPrice{
			{Symbol: "BTCUSDT", Price: "50000.00"},
		},
		shouldError: false,
	}

	logger := slog.Default()

	loader := &BinancePriceLoader{
		config:        &config.Config{},
		binanceClient: mockClient,
		priceManager:  mockManager,
		logger:        logger,
	}

	// This should not block indefinitely due to timeout
	err := loader.LoadPrices([]string{"BTCUSDT"})

	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
}
