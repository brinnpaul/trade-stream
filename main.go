package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"trade-stream/client"
	configure "trade-stream/config"
	"trade-stream/data"
	"trade-stream/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	configPath := "config/config.yaml"
	config, configErr := configure.LoadConfig(configPath)
	if configErr != nil {
		logger.Error("Failed to load config file", "error", configErr)
		return
	}

	priceManager := data.NewInMemoryPriceManager(logger)
	priceManager.Start()

	defer priceManager.Shutdown()

	binanceClient := client.NewBinanceClient(config)
	tradeStreamFactory := client.NewTradeStreamFactory(config, logger)
	priceLoader := service.NewBinancePriceLoader(config, binanceClient, tradeStreamFactory, priceManager, logger)

	var symbols []string
	if config.ShouldLoadAllSymbols() {
		allSymbols, loadPricesErr := priceLoader.LoadAllPrices()
		if loadPricesErr != nil {
			return
		}
		symbols = allSymbols
	} else {
		loadPricesErr := priceLoader.LoadPrices(config.Binance.Symbols)
		symbols = config.Binance.Symbols
		if loadPricesErr != nil {
			return
		}
	}

	mux := SetupRoutes(priceManager, logger)
	server := NewServer("8080", mux, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	streamDone := make(chan struct{})
	go func() {
		defer close(streamDone)
		err := priceLoader.LoadPricesFromStream(ctx, symbols, config.Binance.SymbolsPerStream)
		if err != nil {
			logger.Error("Failed to load prices from stream", "error", err)
		}
	}()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		if err := server.Start(); err != nil {
			logger.Error("Server error", "error", err)
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-shutdown:
		logger.Info("Received shutdown signal", "signal", sig)
	}

	cancel()

	if err := server.Stop(); err != nil {
		logger.Error("Failed to stop server gracefully", "error", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	wg := sync.WaitGroup{}

	// Wait for streams to finish
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-streamDone:
			logger.Info("WebSocket streams stopped")
		case <-shutdownCtx.Done():
			logger.Warn("WebSocket streams shutdown timeout")
		}
	}()

	// Wait for server to finish
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-serverDone:
			logger.Info("HTTP server stopped")
		case <-shutdownCtx.Done():
			logger.Warn("HTTP server shutdown timeout")
		}
	}()

	// Wait for all components to finish
	wg.Wait()

	logger.Info("Graceful shutdown completed")
}
