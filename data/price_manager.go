package data

import (
	"log/slog"
	"sync"
	"trade-stream/models"
)

type InMemoryPriceManager struct {
	priceUpdates  chan models.PriceUpdate
	done          chan struct{}
	currentPrices models.Prices
	subscribers   map[chan models.Prices]bool
	mu            sync.RWMutex
	logger        *slog.Logger
}

type PriceManager interface {
	Subscribe() chan models.Prices
	Unsubscribe(ch chan models.Prices)
	Start()
	Shutdown()
	GetPrices() models.Prices
	GetDoneChannel() chan struct{}
	GetPriceUpdateChannel() chan models.PriceUpdate
}

func NewInMemoryPriceManager(logger *slog.Logger) PriceManager {
	return &InMemoryPriceManager{
		priceUpdates:  make(chan models.PriceUpdate),
		done:          make(chan struct{}),
		subscribers:   make(map[chan models.Prices]bool),
		currentPrices: *models.EmptyPrices(),
		mu:            sync.RWMutex{},
		logger:        logger,
	}
}

// Subscription

func (pm *InMemoryPriceManager) Subscribe() chan models.Prices {
	pm.logger.Debug("Adding Subscriber")
	subscriber := make(chan models.Prices, 1) // buffer to allow for delay in processing

	pm.mu.Lock()
	pm.subscribers[subscriber] = true
	pm.mu.Unlock()

	pm.sendSnapshot(subscriber)

	return subscriber
}

func (pm *InMemoryPriceManager) Unsubscribe(ch chan models.Prices) {
	pm.logger.Debug("Removing Subscriber")
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.subscribers[ch]; exists {
		delete(pm.subscribers, ch)
		pm.logger.Debug("Closing subscriber channel")
		close(ch)
	} else {
		pm.logger.Debug("Subscriber not found in map")
	}
}

func (pm *InMemoryPriceManager) sendSnapshot(subscriber chan models.Prices) {
	subscriber <- pm.currentPrices
}

// Update

func (pm *InMemoryPriceManager) updatePrices() {
	for {
		select {
		case update := <-pm.priceUpdates:
			pm.processUpdate(update)
			pm.notifySubscribers()
		case <-pm.done:
			pm.logger.Debug("Closing price updates")
			return
		}
	}
}

func (pm *InMemoryPriceManager) processUpdate(update models.PriceUpdate) {
	pm.logger.Debug("Received update", "symbol", update.Symbol, "price", update.Price)
	pm.mu.Lock()
	pm.currentPrices.UpdatePriceTicker(update)
	pm.mu.Unlock()
}

func (pm *InMemoryPriceManager) notifySubscribers() {
	// make a copy of subscribers to avoid race conditions
	pm.mu.RLock()
	subscribers := make(map[chan models.Prices]bool)
	for key, value := range pm.subscribers {
		subscribers[key] = value
	}
	pm.mu.RUnlock()

	data := pm.GetPrices()
	for subscriber := range subscribers {
		select {
		case subscriber <- data:
		default:
			pm.logger.Warn("Could not send update to subscriber channel", "num channels", len(pm.subscribers))
		}
	}
}

func (pm *InMemoryPriceManager) GetPrices() models.Prices {
	pm.mu.RLock()
	copiedData := models.CopyPrices(&pm.currentPrices)
	pm.mu.RUnlock()
	return copiedData
}

func (pm *InMemoryPriceManager) GetDoneChannel() chan struct{} {
	return pm.done
}

func (pm *InMemoryPriceManager) GetPriceUpdateChannel() chan models.PriceUpdate {
	return pm.priceUpdates
}

// Start

func (pm *InMemoryPriceManager) Start() {
	pm.logger.Debug("Starting PriceManager")
	go pm.updatePrices()
}

// Shutdown

func (pm *InMemoryPriceManager) Shutdown() {
	pm.logger.Info("Shutting down PriceManager")
	close(pm.done)

	// Close all subscriber channels
	pm.mu.Lock()
	pm.logger.Debug("Closing all subscriber channels", "count", len(pm.subscribers))
	for subscriber := range pm.subscribers {
		pm.logger.Debug("Closing subscriber channel")
		close(subscriber)
	}
	pm.subscribers = nil
	pm.mu.Unlock()
}
