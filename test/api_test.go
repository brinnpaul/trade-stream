package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
	"trade-stream/controllers"
)

func TestPricesApi_GetAllPrices(t *testing.T) {
	// test expects app to be configured for these symbols
	expectedSymbols := map[string]bool{
		"BTCUSD":  true,
		"BTCUSDT": true,
		"ETHUSD":  true,
		"ETHUSDT": true,
	}

	testClient := newTestApiClient()
	priceResponse, err := testClient.GetAllPrices()
	if err != nil {
		t.Errorf("Error getting prices: %v", err)
	}

	for _, priceTicker := range priceResponse.Data {
		_, ok := expectedSymbols[priceTicker.Symbol]
		if !ok {
			t.Errorf("Symbol %s not found in expected symbols", priceTicker.Symbol)
		}
	}
}

func TestPricesApi_GetPricesForSymbols(t *testing.T) {
	symbols := []string{
		"BTCUSD",
		"BTCUSDT",
	}

	expectedSymbols := map[string]bool{
		"BTCUSD":  true,
		"BTCUSDT": true,
	}

	testClient := newTestApiClient()
	priceResponse, err := testClient.GetPricesForSymbols(symbols)
	if err != nil {
		t.Errorf("Error getting prices: %v", err)
	}
	if len(priceResponse.Data) != len(expectedSymbols) {
		t.Error("Expected price data in response to match request")
	}
	for _, priceTicker := range priceResponse.Data {
		_, ok := expectedSymbols[priceTicker.Symbol]
		if !ok {
			t.Errorf("Symbol %s not found in expected symbols", priceTicker.Symbol)
		}
	}
}

type testApiClient struct {
	httpClient *http.Client
	baseUrl    string
}

func newTestApiClient() *testApiClient {
	return &testApiClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseUrl: getTestUrl(),
	}
}

func getTestUrl() string {
	url, found := os.LookupEnv("TEST_SERVICE_URL")
	if found {
		return url
	}

	return "http://localhost:8080"
}

func (tc *testApiClient) GetAllPrices() (*controllers.PriceResponse, error) {
	url := fmt.Sprintf("%s/api/prices", tc.baseUrl)

	resp, err := tc.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	var priceData controllers.PriceResponse
	if err := json.NewDecoder(resp.Body).Decode(&priceData); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &priceData, nil
}

func (tc *testApiClient) GetPricesForSymbols(symbols []string) (*controllers.PriceResponse, error) {
	url := fmt.Sprintf("%s/api/prices?symbols=%s", tc.baseUrl, strings.Join(symbols, ","))

	resp, err := tc.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	var priceData controllers.PriceResponse
	if err := json.NewDecoder(resp.Body).Decode(&priceData); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &priceData, nil
}
