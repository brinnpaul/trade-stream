package controllers

import "trade-stream/models"

type PriceResponse struct {
	Data []models.PriceTicker `json:"data"`
}
