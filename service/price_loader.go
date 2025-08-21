package service

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"
	"trade-stream/client"
	"trade-stream/config"
	"trade-stream/data"
	"trade-stream/models"
)

type BinancePriceLoader struct {
	config             *config.Config
	binanceClient      client.PriceClient
	tradeStreamFactory client.TradeStreamFactory
	priceManager       data.PriceManager
	logger             *slog.Logger
}

type PriceLoader interface {
	LoadPrices(symbols []string) error
	LoadAllPrices() ([]string, error)
	LoadPricesFromStream(symbols []string, symbolsPerStream int) error
}

func NewBinancePriceLoader(
	config *config.Config,
	binanceClient client.PriceClient,
	tradeStreamFactory client.TradeStreamFactory,
	priceManager data.PriceManager,
	logger *slog.Logger,
) PriceLoader {
	return &BinancePriceLoader{
		config:             config,
		binanceClient:      binanceClient,
		tradeStreamFactory: tradeStreamFactory,
		priceManager:       priceManager,
		logger:             logger,
	}
}

func (pl *BinancePriceLoader) LoadPrices(symbols []string) error {
	pl.logger.Debug("Loading prices", "symbols", symbols)
	symbolsSet := createSymbolsSet(symbols)
	var priceSnapShot, err = pl.binanceClient.GetAllLiveTickerPrices()
	if err != nil {
		pl.logger.Error("Error loading prices", "error", err)
		return err
	}

	for _, price := range priceSnapShot {
		symbol := price.Symbol
		_, ok := symbolsSet[symbol]
		if ok {
			update := models.PriceUpdate{
				Symbol: symbol,
				Price:  price.Price,
			}
			select {
			case pl.priceManager.GetPriceUpdateChannel() <- update:
				// Successfully sent
			case <-time.After(100 * time.Millisecond):
				// Timeout - channel blocked for too long
				pl.logger.Warn("Price update timeout for symbol", "symbol", symbol)
			}
		}
	}

	pl.logger.Debug("Finished loading price data")
	return nil
}

func (pl *BinancePriceLoader) LoadAllPrices() ([]string, error) {
	pl.logger.Debug("Loading All prices")
	var priceSnapShot, err = pl.binanceClient.GetAllLiveTickerPrices()
	if err != nil {
		pl.logger.Error("Error loading prices", "error", err)
		return []string{}, err
	}

	var symbols []string
	for _, price := range priceSnapShot {
		symbol := price.Symbol
		symbols = append(symbols, symbol)
		update := models.PriceUpdate{
			Symbol: symbol,
			Price:  price.Price,
		}
		select {
		case pl.priceManager.GetPriceUpdateChannel() <- update:
			// Successfully sent
		case <-time.After(100 * time.Millisecond):
			// Timeout - channel blocked for too long
			pl.logger.Warn("Price update timeout for symbol", "symbol", symbol)
		}
	}

	pl.logger.Debug("Finished loading price data")
	return symbols, nil
}

func createSymbolsSet(symbols []string) map[string]bool {
	result := make(map[string]bool)
	for _, symbol := range symbols {
		result[symbol] = true
	}

	return result
}

func (pl *BinancePriceLoader) LoadPricesFromStream(symbols []string, symbolsPerStream int) error {
	pl.logger.Info("LoadPricesFromStream called", "symbols_count", len(symbols), "symbols", symbols, "symbols_per_stream", symbolsPerStream)

	// Create channels for each stream
	var allDoneChs []chan struct{}
	var allStopChs []chan struct{}

	var i = 0
	for chunk := range slices.Chunk(symbols, symbolsPerStream) {
		i += 1
		streamID := fmt.Sprintf("stream-%d", i)

		pl.logger.Info("Processing chunk", "chunk_number", i, "chunk_symbols", chunk, "chunk_size", len(chunk))

		errHandler := func(err error) {
			pl.logger.Error("Stream error", "stream_id", streamID, "error", err)
		}

		pl.logger.Info("Creating stream", "stream_id", streamID, "symbols", chunk)
		doneCh, stopCh, err := pl.tradeStreamFactory.GetNewStream(
			chunk,
			pl.TradeEventHandler,
			errHandler,
		)

		if err != nil {
			pl.logger.Error("Failed to create stream", "stream_id", streamID, "error", err)
			// Stop all previously created streams
			for _, stopCh := range allStopChs {
				close(stopCh)
			}
			return fmt.Errorf("failed to create stream %s: %w", streamID, err)
		}

		allDoneChs = append(allDoneChs, doneCh)
		allStopChs = append(allStopChs, stopCh)

		pl.logger.Info("Stream started", "stream_id", streamID, "symbols", chunk)
	}

	//aggregatedDone := pl.aggregateChannels(allDoneChs)
	//aggregatedStop := make(chan struct{})

	// Set up signal handling for graceful shutdown
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt signal
	<-interrupt

	// Gracefully shutdown all streams
	for _, stopCh := range allStopChs {
		close(stopCh)
	}

	// Wait for graceful shutdown
	for _, doneCh := range allDoneChs {
		<-doneCh
	}

	return nil
}

func (pl *BinancePriceLoader) TradeEventHandler(event *client.MultiStreamWsTradeEvent) {
	pl.logger.Debug("received trade event", "event", event)
	symbol := getSymbolFromStreamName(event.Stream)
	priceUpdate := &models.PriceUpdate{
		Symbol: symbol,
		Price:  event.Data.Price,
	}
	select {
	case pl.priceManager.GetPriceUpdateChannel() <- *priceUpdate:
	case <-pl.priceManager.GetDoneChannel():
		return
	}
}

// ensure all keys are upper case
func getSymbolFromStreamName(symbol string) string {
	return strings.ToUpper(strings.Split(symbol, "@")[0])
}
