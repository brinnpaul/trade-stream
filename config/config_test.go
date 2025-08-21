package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestLoadConfig_ValidFile(t *testing.T) {
	configContent := `
binance:
  api_url: "https://api.binance.us"
  websocket_url: "wss://stream.binance.us:9443"
  symbols:
    - "BTCUSDT"
    - "ETHUSDT"
  symbols_per_stream: 5
`
	config, err := loadTempConfig(configContent, t)
	if err != nil {
		t.Fatalf("error loading config: %v", err)
		return
	}

	// Verify values
	if config.Binance.ApiUrl != "https://api.binance.us" {
		t.Errorf("Expected ApiUrl 'https://api.binance.us', got '%s'", config.Binance.ApiUrl)
	}

	if config.Binance.WebsocketUrl != "wss://stream.binance.us:9443" {
		t.Errorf("Expected WebsocketUrl 'wss://stream.binance.us:9443', got '%s'", config.Binance.WebsocketUrl)
	}

	if config.Binance.SymbolsPerStream != 5 {
		t.Errorf("Expected SymbolsPerStream '5', got %d", config.Binance.SymbolsPerStream)
	}

	var expectedSymbols = []string{"BTCUSDT", "ETHUSDT"}
	if !reflect.DeepEqual(config.Binance.Symbols, expectedSymbols) {
		t.Errorf(
			"Expected symbols {%s}, got {%s}",
			strings.Join(expectedSymbols, ", "),
			strings.Join(config.Binance.Symbols, ", "))
	}
}

func TestLoadConfig_InvalidFile(t *testing.T) {
	configContent := `
binance:
  api_url: "https://api.binance.us"
  websocket_url: "wss://stream.binance.us:9443"
  symbols: [invalid
`

	_, err := loadTempConfig(configContent, t)

	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestLoadConfig_ShouldLoadAllSymbols(t *testing.T) {
	tests := []struct {
		name                         string
		configContent                string
		expectedShouldLoadAllSymbols bool
	}{
		{
			name: "should load all symbols",
			configContent: `
binance:
  symbols:
`,
			expectedShouldLoadAllSymbols: true,
		},
		{
			name: "should not load all symbols",
			configContent: `
binance:
  symbols:
    - 'BTCUSDT'
    - 'ETHUSDT'
`,
			expectedShouldLoadAllSymbols: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config, err := loadTempConfig(test.configContent, t)
			if err != nil {
				t.Fatalf("error loading config: %v", err)
				return
			}
			result := config.ShouldLoadAllSymbols()
			if result != test.expectedShouldLoadAllSymbols {
				t.Errorf("Expected %t, got %t", test.expectedShouldLoadAllSymbols, result)
			}
		})
	}
}

func loadTempConfig(configContent string, t *testing.T) (*Config, error) {
	tmpFile, err := os.CreateTemp("", "test_config_*.yaml")
	if err != nil {
		t.Fatal("Failed to create temp file:", err)
		return nil, err
	}
	defer os.Remove(tmpFile.Name())

	// Write config content
	if _, err := tmpFile.WriteString(configContent); err != nil {
		return nil, err
	}
	tmpFile.Close()

	// Test loading
	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		return nil, err
	}

	return config, nil
}
