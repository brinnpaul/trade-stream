package main

import (
	"log/slog"
	"os"
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
	server := NewServer(
		"8080",
		mux,
		logger)

	streamCh := make(chan struct{})
	go func() {
		defer close(streamCh)
		err := priceLoader.LoadPricesFromStream(symbols, config.Binance.SymbolsPerStream)
		if err != nil {
			logger.Error("Failed to load prices from stream", "error", err)
			return
		}
	}()

	if err := server.Start(); err != nil {
		logger.Error("Server error:", err)
	}
	<-streamCh
}
