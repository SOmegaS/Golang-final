package models

import (
	"encoding/json"
	"time"
)

type Config struct {
	MongoIRI        string `json:"mongoIRI"`
	OfferingAddress string `json:"offeringAddress"`
	KafkaAddress    string `json:"kafkaAddress"`
	ServeAddress    string `json:"serveAddress"`
	BasePath        string `json:"basePath"`
	CollName        string `json:"collName"`
	DatabaseName    string `json:"databaseName"`
	JaegerAddress   string `json:"jaegerAddress"`
}

type Location struct {
	Lat float64 `bson:"lat"`
	Lng float64 `bson:"lng"`
}

type Price struct {
	Amount   float64 `bson:"amount"`
	Currency string  `bson:"currency"`
}

type Trip struct {
	ID      string   `bson:"id"`
	UserID  string   `bson:"user_id"`
	OfferID string   `bson:"offer_id"`
	From    Location `bson:"from"`
	To      Location `bson:"to"`
	Price   Price    `bson:"price"`
	Status  string   `bson:"status"`
}

type OmitUserTrip struct {
	ID      string   `bson:"id"`
	OfferID string   `bson:"offer_id"`
	From    Location `bson:"from"`
	To      Location `bson:"to"`
	Price   Price    `bson:"price"`
	Status  string   `bson:"status"`
}

type LocationOffering struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type PriceOffering struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type OrderOffering struct {
	From     Location `json:"from"`
	To       Location `json:"to"`
	ClientID string   `json:"client_id"`
	Price    Price    `json:"price"`
}

type Offer struct {
	OfferID string `json:"offer_id"`
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

type CommandCreateData struct {
	OfferId string `json:"offer_id"`
}

type CommandCancelData struct {
	TripId string `json:"trip_id"`
	Reason string `json:"reason"`
}

type EventData struct {
	TripId string `json:"trip_id"`
}
