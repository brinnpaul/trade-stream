package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"trade-stream/config"
)

type BinanceClient struct {
	httpClient *http.Client
	baseURL    string
}

type PriceClient interface {
	GetAllLiveTickerPrices() ([]TickerPrice, error)
}

func NewBinanceClient(config *config.Config) PriceClient {
	return &BinanceClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: config.Binance.ApiUrl,
	}
}

// GetAllLiveTickerPrices retrieves live ticker prices for all symbols
func (c *BinanceClient) GetAllLiveTickerPrices() ([]TickerPrice, error) {
	url := fmt.Sprintf("%s/api/v3/ticker/price", c.baseURL)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tickers []TickerPrice
	if err := json.NewDecoder(resp.Body).Decode(&tickers); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return tickers, nil
}
