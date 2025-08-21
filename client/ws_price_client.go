package client

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"trade-stream/config"
)

type BinanceTradeStreamFactory struct {
	config *config.Config
	logger *slog.Logger
}

type TradeStreamFactory interface {
	GetNewStream(symbols []string, handler MultiStreamWsTradeHandler, errHandler ErrorHandler) (doneCh, stopCh chan struct{}, err error)
}

func NewTradeStreamFactory(config *config.Config, logger *slog.Logger) TradeStreamFactory {
	return &BinanceTradeStreamFactory{
		config: config,
		logger: logger,
	}
}

func (f *BinanceTradeStreamFactory) GetNewStream(symbols []string, handler MultiStreamWsTradeHandler, errHandler ErrorHandler) (doneCh, stopCh chan struct{}, err error) {
	formattedUrlSlug := formatSymbolsUrlSlug(symbols)
	fullWsUrl := fmt.Sprintf("%s/%s", f.config.Binance.WebsocketUrl, formattedUrlSlug)
	streamClient := NewWebsocketStreamClient(fullWsUrl, f.logger)

	wsHandler := func(message []byte) {
		event, err := multiStreamEventMarshaler(message)
		if err != nil {
			errHandler(err)
			return
		}
		handler(event)
	}
	return streamClient.Connect(wsHandler, errHandler)
}

func formatSymbolsUrlSlug(symbols []string) string {
	var fmtSymbols []string

	for _, symbol := range symbols {
		fmtSymbols = append(fmtSymbols, fmt.Sprintf("%s@trade", strings.ToLower(symbol)))
	}

	var streams = strings.Join(fmtSymbols, "/")
	return fmt.Sprintf("stream?streams=%s", streams)
}

func multiStreamEventMarshaler(message []byte) (*MultiStreamWsTradeEvent, error) {
	event := new(MultiStreamWsTradeEvent)
	err := json.Unmarshal(message, event)
	if err != nil {
		return nil, err
	}

	return event, nil
}
