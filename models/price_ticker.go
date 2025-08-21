package models

import (
	"time"
)

// PriceTicker represents a ticker Price with timestamp information
type PriceTicker struct {
	Symbol             string    `json:"symbol"`
	Price              string    `json:"price"`
	LastUpdatedTimeUtc time.Time `json:"lastUpdatedTimeUtc"`
}

// NewPriceTicker creates a new PriceTicker with the current UTC time
func NewPriceTicker(symbol, price string) PriceTicker {
	return PriceTicker{
		Symbol:             symbol,
		Price:              price,
		LastUpdatedTimeUtc: time.Now().UTC(),
	}
}

// UpdatePrice updates the Price and timestamp
func (pt *PriceTicker) UpdatePrice(price string) {
	pt.Price = price
	pt.LastUpdatedTimeUtc = time.Now().UTC()
}

// PriceUpdate update Price for a symbol meant to be passed around channels
type PriceUpdate struct {
	Symbol string
	Price  string
}

// Prices wrapper class for live price_ticker data
type Prices struct {
	data map[string]PriceTicker
}

// EmptyPrices returns a new empty prices class
func EmptyPrices() *Prices {
	return &Prices{
		data: make(map[string]PriceTicker),
	}
}

// CopyPrices return a copy of prices
func CopyPrices(prices *Prices) Prices {
	data := make(map[string]PriceTicker, len(prices.data))
	for k, v := range prices.data {
		data[k] = v
	}
	return Prices{
		data: data,
	}
}

func (p *Prices) GetPriceTicker(symbol string) PriceTicker {
	return p.data[symbol]
}

func (p *Prices) UpdatePriceTicker(update PriceUpdate) {
	ticker, ok := p.data[update.Symbol]
	if ok {
		ticker.UpdatePrice(update.Price)
		p.data[update.Symbol] = ticker
	} else {
		p.data[update.Symbol] = NewPriceTicker(update.Symbol, update.Price)
	}
}

func (p *Prices) GetRawData() map[string]PriceTicker {
	return p.data
}
