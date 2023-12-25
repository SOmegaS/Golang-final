package httpadapter

import (
	"context"
	"encoding/json"
	"final-project/models"
	kfk "final-project/pkg/kafka"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/juju/zaputil/zapctx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"io"
	"log"
	"net/http"
	"time"
)

type adapter struct {
	config *models.Config
	//tracer      trace.Tracer
	mongoClient   *mongo.Client
	mongoColl     *mongo.Collection
	server        *http.Server
	connTrip      *kafka.Conn
	connDriver    *kafka.Conn
	RequestsTotal *prometheus.CounterVec
	ResponseTime  *prometheus.GaugeVec
}

func (a *adapter) ListTrips(w http.ResponseWriter, r *http.Request) {
	// Статистика
	startTime := time.Now()
	defer a.ResponseTime.WithLabelValues("ListTrips").Set(float64(time.Now().Sub(startTime).Nanoseconds()))
	a.RequestsTotal.WithLabelValues("ListTrips").Inc()

	ctx := context.Background()
	userID := r.Header.Get("user_id")
	if userID == "" {
		http.Error(w, "Missing user_id in header", http.StatusBadRequest)
		return
	}

	// Query MongoDB for trips based on user_id
	cursor, err := a.mongoColl.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		http.Error(w, "Finding element error", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	// Decode MongoDB documents into Trip structs
	var trips []models.OmitUserTrip
	for cursor.Next(ctx) {
		var trip models.Trip
		if err := cursor.Decode(&trip); err != nil {
			http.Error(w, "Decoding error", http.StatusInternalServerError)
			return
		}
		respTrip := models.OmitUserTrip{
			ID:      trip.ID,
			OfferID: trip.OfferID,
			From:    trip.From,
			To:      trip.To,
			Price:   trip.Price,
			Status:  trip.Status,
		}
		trips = append(trips, respTrip)
	}

	// Convert trips to JSON
	tripsJSON, err := json.Marshal(trips)
	if err != nil {
		http.Error(w, "Marshaling error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Write the JSON response to the client
	_, err = w.Write(tripsJSON)
	if err != nil {
		http.Error(w, "Writing response error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (a *adapter) CreateTrip(w http.ResponseWriter, r *http.Request) {
	// Статистика
	startTime := time.Now()
	defer a.ResponseTime.WithLabelValues("CreateTrip").Set(float64(time.Now().Sub(startTime).Nanoseconds()))
	a.RequestsTotal.WithLabelValues("CreateTrip").Inc()

	ctx := context.Background()

	// Retrieve user_id from header
	userID := r.Header.Get("user_id")
	if userID == "" {
		http.Error(w, "Missing user_id in header", http.StatusBadRequest)
		return
	}

	var incomingOffer models.Offer
	err := json.NewDecoder(r.Body).Decode(&incomingOffer)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	resp, err := http.Get(a.config.OfferingAddress + "/" + incomingOffer.OfferID)
	if err != nil {
		http.Error(w, "Failed connecting to offering service", http.StatusInternalServerError)
		return
	}
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Error reading response body", http.StatusBadRequest)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer resp.Body.Close()

	var decodedOrder models.OrderOffering
	err = json.Unmarshal(bytes, &decodedOrder)
	fmt.Println(decodedOrder)
	if err != nil {
		http.Error(w, "Error decoding JSON request", http.StatusBadRequest)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if userID != decodedOrder.ClientID {
		http.Error(w, "Wrong user_id", http.StatusBadRequest)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	newID := uuid.New().String()
	// Insert the offer_id into MongoDB
	newTrip := models.Trip{
		ID:      newID,
		UserID:  userID,
		OfferID: incomingOffer.OfferID,
		From: models.Location{
			Lat: decodedOrder.From.Lat,
			Lng: decodedOrder.From.Lng,
		},
		To: models.Location{
			Lat: decodedOrder.To.Lat,
			Lng: decodedOrder.To.Lng,
		},
		Price: models.Price{
			Amount:   decodedOrder.Price.Amount,
			Currency: decodedOrder.Price.Currency,
		},
		Status: "DRIVER_SEARCH",
	}

	//make kafka payload
	kafkaPayload := models.Request{
		Id:              newID,
		Source:          "/client",
		Type:            "trip.command.create",
		DataContentType: "application/json",
		Time:            time.Now().UTC(),
		Data:            nil,
	}

	createTripData := models.CommandCreateData{
		OfferId: incomingOffer.OfferID,
	}

	kafkaPayload.Data, err = json.Marshal(createTripData)
	if err != nil {
		http.Error(w, "Kafka payload generating error", http.StatusInternalServerError)
		return
	}

	kafkaPayloadJSON, err := json.Marshal(kafkaPayload)
	if err != nil {
		http.Error(w, "Error encoding Kafka payload", http.StatusInternalServerError)
		return
	}

	err = kfk.SendToTopic(a.connDriver, kafkaPayloadJSON)
	if err != nil {
		log.Println("Error sending message to Kafka:", err)
		http.Error(w, "Error sending message to Kafka", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	insertResult, err := a.mongoColl.InsertOne(ctx, newTrip)
	if err != nil {
		http.Error(w, "Insertion error", http.StatusInternalServerError)
		return
	}

	// Return the inserted document ID
	fmt.Fprintf(w, "Inserted document ID: %v", insertResult.InsertedID)
	w.WriteHeader(http.StatusOK)
}

func (a *adapter) GetTripByID(w http.ResponseWriter, r *http.Request) {
	// Статистика
	startTime := time.Now()
	defer a.ResponseTime.WithLabelValues("GetTripByID").Set(float64(time.Now().Sub(startTime).Nanoseconds()))
	a.RequestsTotal.WithLabelValues("GetTripByID").Inc()

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
	var trip models.Trip
	err := a.mongoColl.FindOne(ctx, filter).Decode(&trip)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Trip not found", http.StatusNotFound)
			return
		}

		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	respTrip := models.OmitUserTrip{
		ID:      trip.ID,
		OfferID: trip.OfferID,
		From:    trip.From,
		To:      trip.To,
		Price:   trip.Price,
		Status:  trip.Status,
	}

	// Marshal the result to JSON and send it in the response
	tripJSON, err := json.Marshal(respTrip)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(tripJSON)
}

func (a *adapter) CancelTrip(w http.ResponseWriter, r *http.Request) {
	// Статистика
	startTime := time.Now()
	defer a.ResponseTime.WithLabelValues("CancelTrip").Set(float64(time.Now().Sub(startTime).Nanoseconds()))
	a.RequestsTotal.WithLabelValues("CancelTrip").Inc()

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

	//make kafka payload
	kafkaPayload := models.Request{
		Id:              uuid.New().String(),
		Source:          "/client",
		Type:            "trip.command.create",
		DataContentType: "application/json",
		Time:            time.Now().UTC(),
		Data:            nil,
	}

	cancelTripData := models.CommandCancelData{
		TripId: tripID,
		Reason: reason,
	}

	kafkaPayloadData, err := json.Marshal(cancelTripData)
	if err != nil {
		http.Error(w, "Kafka payload generating error", http.StatusInternalServerError)
		return
	}

	kafkaPayload.Data = kafkaPayloadData

	kafkaPayloadJSON, err := json.Marshal(kafkaPayload)
	if err != nil {
		http.Error(w, "Error encoding Kafka payload", http.StatusInternalServerError)
		return
	}

	err = kfk.SendToTopic(a.connDriver, kafkaPayloadJSON)
	if err != nil {
		log.Println("Error sending message to Kafka:", err)
		http.Error(w, "Error sending message to Kafka", http.StatusInternalServerError)
		return
	}

	// Update the document in the collection
	updateResult, err := a.mongoColl.UpdateOne(ctx, filter, update)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if updateResult.ModifiedCount == 0 {
		http.Error(w, "Trip not found or not authorized", http.StatusNotFound)
		return
	}

	// Respond with a success message or the updated document ID
	fmt.Fprintf(w, "Trip canceled successfully")
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

	go func() {
		for {
			select {
			case <-ctx.Done():
				return // Exit the goroutine if the context is canceled
			default:
				// Read and process Kafka messages
				a.iteration(ctx)
				time.Sleep(1 * time.Second)
			}
		}
	}()

	// Start the HTTP server
	a.server = &http.Server{Addr: a.config.ServeAddress, Handler: apiRouter}
	return a.server.ListenAndServe()
}

func (a *adapter) Shutdown(ctx context.Context) {
	_ = a.server.Shutdown(ctx)
}

func (a *adapter) iteration(ctx context.Context) {
	logger := zapctx.Logger(ctx)
	// Чтение из Kafka
	bytes, err := kfk.ReadFromTopic(a.connTrip)
	if err != nil {
		logger.Error("Kafka read error. %v", zap.Error(err))
		return
	}
	logger.Info("Message detected")

	// Десериализация запроса
	var request models.Request
	err = json.Unmarshal(bytes, &request)
	if err != nil {
		logger.Error("Unmarshal error. %v", zap.Error(err))
		return
	}

	// Проверка на тип Data
	if request.DataContentType != "application/json" {
		logger.Error("Data type error. %v", zap.Error(err))
		return
	}
	var eventData models.EventData
	err = json.Unmarshal(request.Data, &eventData)
	if err != nil {
		logger.Error("Unmarshal error. %v", zap.Error(err))
		return
	}
	filter := bson.M{"id": eventData.TripId}
	update := bson.M{"$set": bson.M{"status": selectStatus(request.Type)}}

	_, err = a.mongoColl.UpdateOne(ctx, filter, update)
	if err != nil {
		logger.Error("MongoDB update error. %v", zap.Error(err))
		return
	}
	logger.Info("MongoDB updated")
}

func selectStatus(eventType string) string {
	var newStatus string
	switch eventType {
	case "trip.event.accepted":
		newStatus = "ACCEPTED"
	case "trip.event.ended":
		newStatus = "ENDED"
	case "trip.event.started":
		newStatus = "STARTED"
	}
	return newStatus
}

func New(ctx context.Context, config *models.Config, client *mongo.Client, connTrip *kafka.Conn, connDriver *kafka.Conn,
	requestsTotal *prometheus.CounterVec, responseTime *prometheus.GaugeVec) Adapter {
	return &adapter{
		config:      config,
		mongoClient: client,
		mongoColl:   client.Database(config.DatabaseName).Collection(config.CollName),
		connTrip:    connTrip,
		connDriver:  connDriver,
		//tracer:      ctx.Value("tracer").(trace.Tracer),
		RequestsTotal: requestsTotal,
		ResponseTime:  responseTime,
	}
}
