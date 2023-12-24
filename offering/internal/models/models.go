package models

import "crypto/rsa"

type Location struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type Price struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type Order struct {
	From     Location `json:"from"`
	To       Location `json:"to"`
	ClientID string   `json:"client_id"`
	Price    Price    `json:"price"`
}

type Config struct {
	PrivateKey rsa.PrivateKey `json:"privateKey"`
}
