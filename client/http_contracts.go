package client

// TickerPrice represents the live ticker price response
type TickerPrice struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}
