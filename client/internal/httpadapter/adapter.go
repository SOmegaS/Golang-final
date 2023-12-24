package httpadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/juju/zaputil/zapctx"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"io"
	"log"
	"net/http"
	"time"
)

type adapter struct {
	config *Config
	//tracer      trace.Tracer
	mongoClient *mongo.Client
	server      *http.Server
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

type LocationOffering struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type PriceOffering struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type OrderOffering struct {
	//ID		 string   `json:"id"`
	From     Location `json:"from"`
	To       Location `json:"to"`
	ClientID string   `json:"client_id"`
	Price    Price    `json:"price"`
}

type Offer struct {
	OfferID string `json:"offer_id"`
}

func (a *adapter) ListTrips(w http.ResponseWriter, r *http.Request) {
	collection := a.mongoClient.Database("my_mongo").Collection("trips")
	ctx := context.Background()
	userID := r.Header.Get("user_id")
	if userID == "" {
		http.Error(w, "Missing user_id in header", http.StatusBadRequest)
		return
	}

	// Query MongoDB for trips based on user_id
	cursor, err := collection.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	// Decode MongoDB documents into Trip structs
	var trips []Trip
	for cursor.Next(ctx) {
		var trip Trip
		if err := cursor.Decode(&trip); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		trips = append(trips, trip)
	}

	// Convert trips to JSON
	tripsJSON, err := json.Marshal(trips)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Write the JSON response to the client
	_, err = w.Write(tripsJSON)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (a *adapter) CreateTrip(w http.ResponseWriter, r *http.Request) {
	collection := a.mongoClient.Database("my_mongo").Collection("trips")
	ctx := context.Background()

	// Retrieve user_id from header
	userID := r.Header.Get("user_id")
	if userID == "" {
		http.Error(w, "Missing user_id in header", http.StatusBadRequest)
		return
	}

	var incomingOffer Offer
	err := json.NewDecoder(r.Body).Decode(&incomingOffer)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	resp, err := http.Get("http://localhost:8888/offers/" + incomingOffer.OfferID)
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Error reading response body", http.StatusBadRequest)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var decodedOrder OrderOffering
	err = json.Unmarshal(bytes, &decodedOrder)
	fmt.Println(decodedOrder)
	if err != nil {
		http.Error(w, "Error decoding JSON request", http.StatusBadRequest)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer resp.Body.Close()
	// Insert the offer_id into MongoDB
	newTrip := Trip{
		//ID:      decodedOrder.ID,
		UserID:  userID,
		OfferID: incomingOffer.OfferID,
		From: Location{
			Lat: decodedOrder.From.Lat,
			Lng: decodedOrder.From.Lng,
		},
		To: Location{
			Lat: decodedOrder.To.Lat,
			Lng: decodedOrder.To.Lng,
		},
		Price: Price{
			Amount:   decodedOrder.Price.Amount,
			Currency: decodedOrder.Price.Currency,
		},
		Status: "DRIVER_SEARCH",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	insertResult, err := collection.InsertOne(ctx, newTrip)
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Return the inserted document ID
	fmt.Fprintf(w, "Inserted document ID: %v", insertResult.InsertedID)
	w.WriteHeader(http.StatusOK)
}

func (a *adapter) GetTripByID(w http.ResponseWriter, r *http.Request) {
	collection := a.mongoClient.Database("my_mongo").Collection("trips")
	ctx := context.Background()

	// Retrieve user_id from header
	userID := r.Header.Get("user_id")
	if userID == "" {
		http.Error(w, "Missing user_id in header", http.StatusBadRequest)
		return
	}

	tripID := chi.URLParam(r, "trip_id")
	if tripID == "" {
		http.Error(w, "Missing trip_id in URL parameters", http.StatusBadRequest)
		return
	}

	// Define a filter to find the document by the "id" key
	filter := bson.M{"id": tripID, "user_id": userID}

	// Find the document in the collection
	var trip Trip
	err := collection.FindOne(ctx, filter).Decode(&trip)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Trip not found", http.StatusNotFound)
			return
		}

		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Marshal the result to JSON and send it in the response
	tripJSON, err := json.Marshal(trip)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(tripJSON)
}

func (a *adapter) CancelTrip(w http.ResponseWriter, r *http.Request) {
	collection := a.mongoClient.Database("my_mongo").Collection("trips")
	ctx := context.Background()

	// Retrieve user_id from header
	userID := r.Header.Get("user_id")
	if userID == "" {
		http.Error(w, "Missing user_id in header", http.StatusBadRequest)
		return
	}

	tripID := chi.URLParam(r, "trip_id")
	if tripID == "" {
		http.Error(w, "Missing trip_id in URL parameters", http.StatusBadRequest)
		return
	}

	// Retrieve "reason" from query parameters
	reason := r.URL.Query().Get("reason")
	if reason == "" {
		http.Error(w, "Missing reason in query parameters", http.StatusBadRequest)
		return
	}

	// Define a filter to find the document by the "id" key
	filter := bson.M{"id": tripID, "user_id": userID}

	// Define an update to set the "status" field to "CANCELED"
	update := bson.M{
		"$set": bson.M{
			"status": "CANCELED",
		},
	}

	// Update the document in the collection
	updateResult, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if updateResult.ModifiedCount == 0 {
		http.Error(w, "Trip not found or not authorized", http.StatusNotFound)
		return
	}

	// Respond with a success message or the updated document ID
	fmt.Fprintf(w, "Trip canceled successfully. Updated document ID: %v", updateResult.UpsertedID)
}

func (a *adapter) Serve(ctx context.Context) error {
	logger := zapctx.Logger(ctx)
	logger.Info("ready to serve")
	apiRouter := chi.NewRouter()
	//apiRouter.Handle("/metrics", promhttp.Handler())

	//counter := promauto.NewCounter(prometheus.CounterOpts{
	//	Namespace: "hw4", Name: "testcounter", Help: "Testing endpoint request counter",
	//})

	apiRouter.Get("/trips", http.HandlerFunc(a.ListTrips))
	apiRouter.Post("/trips", http.HandlerFunc(a.CreateTrip))
	apiRouter.Get("/trips/{trip_id}", http.HandlerFunc(a.GetTripByID))
	apiRouter.Post("/trips/{trip_id}/cancel", http.HandlerFunc(a.CancelTrip))
	apiRouter.Get("/", func(w http.ResponseWriter, r *http.Request) { // testing endpoint
		w.WriteHeader(http.StatusOK)
	})

	apiRouter.Mount(a.config.BasePath, apiRouter)

	a.server = &http.Server{Addr: a.config.ServeAddress, Handler: apiRouter}

	return a.server.ListenAndServe()
}

func (a *adapter) Shutdown(ctx context.Context) {
	_ = a.server.Shutdown(ctx)
}

func New(ctx context.Context, config *Config, client *mongo.Client) Adapter {
	return &adapter{
		config:      config,
		mongoClient: client,
		//tracer:      ctx.Value("tracer").(trace.Tracer),
	}
}
