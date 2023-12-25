package models

import (
	"encoding/json"
	"time"
)

type Config struct {
	KafkaAddress    string `json:"kafkaAddress"`
	OfferingAddress string `json:"offeringAddress"`
}

type Order struct {
	From     Location `json:"from"`
	To       Location `json:"to"`
	ClientID string   `json:"client_id"`
	Price    Price    `json:"price"`
}

type Request struct {
	Id              string          `json:"id"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	DataContentType string          `json:"datacontenttype"`
	Time            time.Time       `json:"time"`
	Data            json.RawMessage `json:"data"`
}

type Data interface{}

type CommandAcceptData struct {
	TripId   string `json:"trip_id"`
	DriverId string `json:"driver_id"`
}

type CommandCancelData struct {
	TripId string `json:"trip_id"`
	Reason string `json:"reason"`
}

type CommandCreateData struct {
	OfferId string `json:"offer_id"`
}

type CommandEndData struct {
	TripId string `json:"trip_id"`
}

type CommandStartData struct {
	TripId string `json:"trip_id"`
}

type EventAcceptData struct {
	TripId string `json:"trip_id"`
}

type EventCancelData struct {
	TripId string `json:"trip_id"`
}

type Location struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type Price struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type EventCreateData struct {
	TripId  string   `json:"trip_id"`
	OfferId string   `json:"offer_id"`
	Price   Price    `json:"price"`
	Status  string   `json:"status"`
	From    Location `json:"from"`
	To      Location `json:"to"`
}

type EventEndData struct {
	TripId string `json:"trip_id"`
}

type EventStartData struct {
	TripId string `json:"trip_id"`
}
