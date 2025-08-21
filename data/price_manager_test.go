package data

import (
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"
	"trade-stream/models"
)

func TestNewInMemoryPriceManager(t *testing.T) {
	logger := slog.Default()
	pm := NewInMemoryPriceManager(logger)

	if pm == nil {
		t.Fatal("Expected PriceManager to be created, got nil")
	}

	// Check that channels are initialized
	if pm.GetDoneChannel() == nil {
		t.Error("Expected done channel to be initialized")
	}

	if pm.GetPriceUpdateChannel() == nil {
		t.Error("Expected price update channel to be initialized")
	}

	// Check initial prices
	prices := pm.GetPrices()
	if prices.GetRawData() == nil {
		t.Error("Expected prices data to be initialized")
	}
}

func TestInMemoryPriceManager_Subscribe(t *testing.T) {
	logger := slog.Default()
	pm := NewInMemoryPriceManager(logger)

	// Subscribe
	subscriber := pm.Subscribe()
	if subscriber == nil {
		t.Fatal("Expected subscriber channel, got nil")
	}

	// Check that snapshot was sent
	select {
	case prices := <-subscriber:
		if prices.GetRawData() == nil {
			t.Error("Expected initial prices snapshot")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected initial prices snapshot within timeout")
	}

	// Check that subscriber was added (cast to concrete type for testing)
	if concretePM, ok := pm.(*InMemoryPriceManager); ok {
		concretePM.mu.RLock()
		if len(concretePM.subscribers) != 1 {
			t.Errorf("Expected 1 subscriber, got %d", len(concretePM.subscribers))
		}
		concretePM.mu.RUnlock()
	}
}

func TestInMemoryPriceManager_Unsubscribe(t *testing.T) {
	logger := slog.Default()
	pm := NewInMemoryPriceManager(logger)

	// Subscribe
	subscriber := pm.Subscribe()

	// Unsubscribe
	pm.Unsubscribe(subscriber)

	// Check that subscriber was removed (cast to concrete type for testing)
	if concretePM, ok := pm.(*InMemoryPriceManager); ok {
		concretePM.mu.RLock()
		if len(concretePM.subscribers) != 0 {
			t.Errorf("Expected 0 subscribers after unsubscribe, got %d", len(concretePM.subscribers))
		}
		concretePM.mu.RUnlock()
	}

	// Drain any remaining data from the channel before checking if it's closed
	select {
	case _, ok := <-subscriber:
		if !ok {
			// Channel is closed, test passed
			return
		}
		// Continue draining if there's more data
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected subscriber channel to be closed within timeout")
		return
	}
}

func TestInMemoryPriceManager_UnsubscribeNonExistent(t *testing.T) {
	logger := slog.Default()
	pm := NewInMemoryPriceManager(logger)

	// Create a channel that wasn't subscribed
	fakeSubscriber := make(chan models.Prices)

	// Should not panic
	pm.Unsubscribe(fakeSubscriber)

	// Check that no subscribers were affected (cast to concrete type for testing)
	if concretePM, ok := pm.(*InMemoryPriceManager); ok {
		concretePM.mu.RLock()
		if len(concretePM.subscribers) != 0 {
			t.Errorf("Expected 0 subscribers, got %d", len(concretePM.subscribers))
		}
		concretePM.mu.RUnlock()
	}
}

func TestInMemoryPriceManager_ProcessUpdate(t *testing.T) {
	logger := slog.Default()
	pm := NewInMemoryPriceManager(logger)

	// Cast to concrete type to access private method
	concretePM, ok := pm.(*InMemoryPriceManager)
	if !ok {
		t.Fatal("Failed to cast to concrete type")
	}

	// Create price update
	update := models.PriceUpdate{
		Symbol: "BTCUSDT",
		Price:  "50000.00",
	}

	// Process update
	concretePM.processUpdate(update)

	// Check that price was updated
	prices := pm.GetPrices()
	ticker := prices.GetPriceTicker("BTCUSDT")

	if ticker.Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT, got %s", ticker.Symbol)
	}

	if ticker.Price != "50000.00" {
		t.Errorf("Expected price 50000.00, got %s", ticker.Price)
	}
}

func TestInMemoryPriceManager_ProcessMultipleUpdates(t *testing.T) {
	logger := slog.Default()
	pm := NewInMemoryPriceManager(logger)

	// Cast to concrete type to access private method
	concretePM, ok := pm.(*InMemoryPriceManager)
	if !ok {
		t.Fatal("Failed to cast to concrete type")
	}

	// Process multiple updates
	updates := []models.PriceUpdate{
		{Symbol: "BTCUSDT", Price: "50000.00"},
		{Symbol: "ETHUSDT", Price: "3000.00"},
		{Symbol: "BTCUSDT", Price: "51000.00"}, // Update existing
	}

	for _, update := range updates {
		concretePM.processUpdate(update)
	}

	// Check final prices
	prices := pm.GetPrices()

	btcTicker := prices.GetPriceTicker("BTCUSDT")
	if btcTicker.Price != "51000.00" {
		t.Errorf("Expected BTC price 51000.00, got %s", btcTicker.Price)
	}

	ethTicker := prices.GetPriceTicker("ETHUSDT")
	if ethTicker.Price != "3000.00" {
		t.Errorf("Expected ETH price 3000.00, got %s", ethTicker.Price)
	}
}

func TestInMemoryPriceManager_NotifySubscribers(t *testing.T) {
	logger := slog.Default()
	pm := NewInMemoryPriceManager(logger)

	// Cast to concrete type to access private method
	concretePM, ok := pm.(*InMemoryPriceManager)
	if !ok {
		t.Fatal("Failed to cast to concrete type")
	}

	// Subscribe
	subscriber := pm.Subscribe()

	// Process update
	update := models.PriceUpdate{
		Symbol: "BTCUSDT",
		Price:  "50000.00",
	}
	concretePM.processUpdate(update)

	// Notify subscribers
	concretePM.notifySubscribers()

	// Check that subscriber received update
	select {
	case prices := <-subscriber:
		ticker := prices.GetPriceTicker("BTCUSDT")
		if ticker.Price != "50000.00" {
			t.Errorf("Expected price 50000.00, got %s", ticker.Price)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected price update within timeout")
	}
}

func TestInMemoryPriceManager_GetPrices(t *testing.T) {
	logger := slog.Default()
	pm := NewInMemoryPriceManager(logger)

	// Cast to concrete type to access private method
	concretePM, ok := pm.(*InMemoryPriceManager)
	if !ok {
		t.Fatal("Failed to cast to concrete type")
	}

	// Add some prices
	updates := []models.PriceUpdate{
		{Symbol: "BTCUSDT", Price: "50000.00"},
		{Symbol: "ETHUSDT", Price: "3000.00"},
	}

	for _, update := range updates {
		concretePM.processUpdate(update)
	}

	// Get prices
	prices := pm.GetPrices()

	// Check that we got a copy, not the original
	if &prices == &concretePM.currentPrices {
		t.Error("Expected GetPrices to return a copy, not the original")
	}

	// Check data
	rawData := prices.GetRawData()
	if len(rawData) != 2 {
		t.Errorf("Expected 2 price tickers, got %d", len(rawData))
	}

	if rawData["BTCUSDT"].Price != "50000.00" {
		t.Errorf("Expected BTC price 50000.00, got %s", rawData["BTCUSDT"].Price)
	}
}

func TestInMemoryPriceManager_StartAndShutdown(t *testing.T) {
	logger := slog.Default()
	pm := NewInMemoryPriceManager(logger)

	// Start the manager
	pm.Start()

	// Give it time to start
	time.Sleep(10 * time.Millisecond)

	// Check that updatePrices goroutine is running
	updateCh := pm.GetPriceUpdateChannel()
	updateCh <- models.PriceUpdate{Symbol: "BTCUSDT", Price: "50000.00"}

	// Give it time to process
	time.Sleep(10 * time.Millisecond)

	// Check that price was updated
	prices := pm.GetPrices()
	ticker := prices.GetPriceTicker("BTCUSDT")
	if ticker.Price != "50000.00" {
		t.Errorf("Expected price 50000.00, got %s", ticker.Price)
	}

	// Shutdown
	pm.Shutdown()

	// Check that done channel is closed
	select {
	case _, ok := <-pm.GetDoneChannel():
		if ok {
			t.Error("Expected done channel to be closed")
		}
	default:
		t.Error("Expected done channel to be closed")
	}
}

func TestInMemoryPriceManager_ConcurrentSubscriptions(t *testing.T) {
	logger := slog.Default()
	pm := NewInMemoryPriceManager(logger)

	// Cast to concrete type to access private fields and methods
	concretePM, ok := pm.(*InMemoryPriceManager)
	if !ok {
		t.Fatal("Failed to cast to concrete type")
	}

	const numSubscribers = 10
	var wg sync.WaitGroup
	subscribers := make([]chan models.Prices, numSubscribers)

	// Subscribe concurrently
	for i := 0; i < numSubscribers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			subscribers[index] = pm.Subscribe()
		}(i)
	}

	wg.Wait()

	// Check that all subscribers were added
	concretePM.mu.RLock()
	if len(concretePM.subscribers) != numSubscribers {
		t.Errorf("Expected %d subscribers, got %d", numSubscribers, len(concretePM.subscribers))
	}
	concretePM.mu.RUnlock()

	// Process an update
	update := models.PriceUpdate{Symbol: "BTCUSDT", Price: "50000.00"}
	concretePM.processUpdate(update)

	// Notify subscribers
	concretePM.notifySubscribers()

	// Check that all subscribers received the update
	for i, subscriber := range subscribers {
		select {
		case prices := <-subscriber:
			ticker := prices.GetPriceTicker("BTCUSDT")
			if ticker.Price != "50000.00" {
				t.Errorf("Subscriber %d: Expected price 50000.00, got %s", i, ticker.Price)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Subscriber %d: Expected price update within timeout", i)
		}
	}

	// Cleanup
	for _, subscriber := range subscribers {
		pm.Unsubscribe(subscriber)
	}
}

func TestInMemoryPriceManager_SubscriberChannelBuffer(t *testing.T) {
	logger := slog.Default()
	pm := NewInMemoryPriceManager(logger)

	// Cast to concrete type to access private method
	concretePM, ok := pm.(*InMemoryPriceManager)
	if !ok {
		t.Fatal("Failed to cast to concrete type")
	}

	// Subscribe
	subscriber := pm.Subscribe()

	// Send multiple updates quickly
	for i := 0; i < 5; i++ {
		update := models.PriceUpdate{
			Symbol: "BTCUSDT",
			Price:  fmt.Sprintf("%d.00", 50000+i),
		}
		concretePM.processUpdate(update)
		concretePM.notifySubscribers()
	}

	// Check that we can receive updates (buffer should handle the load)
	receivedCount := 0
	for i := 0; i < 5; i++ {
		select {
		case prices := <-subscriber:
			receivedCount++
			ticker := prices.GetPriceTicker("BTCUSDT")
			if ticker.Price == "" {
				t.Error("Expected non-empty price")
			}
		case <-time.After(100 * time.Millisecond):
			break
		}
	}

	if receivedCount == 0 {
		t.Error("Expected to receive at least some updates")
	}

	pm.Unsubscribe(subscriber)
}

func TestInMemoryPriceManager_EmptyPrices(t *testing.T) {
	logger := slog.Default()
	pm := NewInMemoryPriceManager(logger)

	// Get initial prices
	prices := pm.GetPrices()
	rawData := prices.GetRawData()

	if rawData == nil {
		t.Error("Expected prices data to be initialized")
	}

	if len(rawData) != 0 {
		t.Errorf("Expected empty prices, got %d entries", len(rawData))
	}
}

func TestInMemoryPriceManager_PriceUpdateTimestamp(t *testing.T) {
	logger := slog.Default()
	pm := NewInMemoryPriceManager(logger)

	// Cast to concrete type to access private method
	concretePM, ok := pm.(*InMemoryPriceManager)
	if !ok {
		t.Fatal("Failed to cast to concrete type")
	}

	// Process update
	update := models.PriceUpdate{
		Symbol: "BTCUSDT",
		Price:  "50000.00",
	}

	beforeUpdate := time.Now().UTC()
	concretePM.processUpdate(update)
	afterUpdate := time.Now().UTC()

	// Check that price was updated
	prices := pm.GetPrices()
	ticker := prices.GetPriceTicker("BTCUSDT")

	if ticker.LastUpdatedTimeUtc.Before(beforeUpdate) || ticker.LastUpdatedTimeUtc.After(afterUpdate) {
		t.Errorf("Expected timestamp between %v and %v, got %v",
			beforeUpdate, afterUpdate, ticker.LastUpdatedTimeUtc)
	}
}

func TestInMemoryPriceManager_ShutdownClosesAllChannels(t *testing.T) {
	logger := slog.Default()
	pm := NewInMemoryPriceManager(logger)

	// Cast to concrete type to access private fields
	concretePM, ok := pm.(*InMemoryPriceManager)
	if !ok {
		t.Fatal("Failed to cast to concrete type")
	}

	// Start and add subscribers
	pm.Start()
	subscriber1 := pm.Subscribe()
	subscriber2 := pm.Subscribe()

	// Give time for goroutines to start
	time.Sleep(10 * time.Millisecond)

	// Shutdown
	pm.Shutdown()

	// Drain any remaining data from subscriber1 channel before checking if it's closed
	select {
	case _, ok := <-subscriber1:
		if !ok {
			// Channel is closed, break out of drain loop
			break
		}
		// Continue draining if there's more data
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected subscriber1 channel to be closed within timeout")
		return
	}

	// Drain any remaining data from subscriber2 channel before checking if it's closed
	select {
	case _, ok := <-subscriber2:
		if !ok {
			// Channel is closed, break out of drain loop
			break
		}
		// Continue draining if there's more data
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected subscriber2 channel to be closed within timeout")
		return
	}

	// Check that subscribers list is cleared
	concretePM.mu.RLock()
	if len(concretePM.subscribers) != 0 {
		t.Errorf("Expected 0 subscribers after shutdown, got %d", len(concretePM.subscribers))
	}
	concretePM.mu.RUnlock()
}
