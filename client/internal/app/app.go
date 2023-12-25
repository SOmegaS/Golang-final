package app

import (
	"context"
	"encoding/json"
	"errors"
	"final-project/internal/httpadapter"
	"final-project/models"
	kfk "final-project/pkg/kafka"
	"github.com/go-chi/chi/v5"
	"github.com/juju/zaputil/zapctx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type app struct {
	//tracer opentracing.Tracer
	httpAdapter   httpadapter.Adapter
	client        *mongo.Client
	fromTripTopic *kafka.Conn
	toDriverTopic *kafka.Conn
}

const configPath = "./config/config.json"

func (a *app) Serve(ctx context.Context) error {
	done := make(chan os.Signal, 1)
	logger := zapctx.Logger(ctx)
	signal.Notify(done, syscall.SIGTERM, syscall.SIGINT)

	logger.Info("starting http server")
	go func() {
		if err := a.httpAdapter.Serve(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server error", zap.Error(err))
			log.Fatal(err.Error())
		}
	}()
	<-done

	log.Println("Shutting down the server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	a.DisconnectMongo()
	a.Shutdown()
	return nil
}

func (a *app) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	a.httpAdapter.Shutdown(ctx)
}

// подключение к Mongo
func initMongo(ctx context.Context, config *models.Config) (*mongo.Client, error) {
	logger := zapctx.Logger(ctx)
	logger.Info("Making MongoDB client")
	client, err := mongo.NewClient(options.Client().ApplyURI(config.MongoIRI))
	if err != nil {
		logger.Error("Failed to create a client", zap.Error(err))
		log.Fatal(err)
	}
	ctx2, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Info("Connecting to MongoDB")
	err = client.Connect(ctx2)
	if err != nil {
		logger.Error("Failed to connect", zap.Error(err))
		log.Fatal(err)
	}
	err = client.Ping(ctx, nil)
	if err != nil {
		logger.Error("Failed to ping MongoDB:", zap.Error(err))
	}
	return client, nil
}

func (a *app) DisconnectMongo() {
	if a.client != nil {
		if err := a.client.Disconnect(context.Background()); err != nil {
			log.Println("Error disconnecting MongoDB:", err)
		}
	}
}

func initConfig() (*models.Config, error) {
	// Получение информации о файле
	stat, err := os.Stat(configPath)
	if err != nil {
		return nil, err
	}

	// Открытие файла
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}

	// Считывание bytes
	data := make([]byte, stat.Size())
	_, err = file.Read(data)
	if err != nil {
		return nil, err
	}

	// Десериализация в конфиг
	var config models.Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func New(ctx context.Context) (App, error) {
	logger := zapctx.Logger(ctx)

	logger.Info("Initializing config")
	config, err := initConfig()
	if err != nil {
		logger.Error("Config init error. %v", zap.Error(err))
		log.Fatal(err)
	}

	client, err := initMongo(ctx, config)
	if err != nil {
		logger.Error("Init app error", zap.Error(err))
		log.Fatal(err)
	}

	// Prometheus
	logger.Info("Initializing Prometheus")
	requestsTotal, responseTime := initPrometheus()
	logger.Info("Prometheus initialized")

	// Подключение к Kafka
	connTrip, err := kfk.ConnectKafka(ctx, config.KafkaAddress, "trip-client-topic", 0)
	if err != nil {
		logger.Error("Kafka connect error. %v", zap.Error(err))
		log.Fatal(err)
	}

	connDriver, err := kfk.ConnectKafka(ctx, config.KafkaAddress, "driver-client-trip-topic", 0)
	if err != nil {
		logger.Error("Kafka connect error. %v", zap.Error(err))
		log.Fatal(err)
	}

	a := &app{
		httpAdapter:   httpadapter.New(ctx, config, client, connTrip, connDriver, requestsTotal, responseTime),
		client:        client,
		fromTripTopic: connTrip,
		toDriverTopic: connDriver,
	}

	return a, nil
}

// initPrometheus инициализация переменных Prometheus
func initPrometheus() (*prometheus.CounterVec, *prometheus.GaugeVec) {
	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method"},
	)
	prometheus.MustRegister(requestsTotal)

	responseTime := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "response_time",
			Help: "Response time of HTTP requests",
		},
		[]string{"method"},
	)
	prometheus.MustRegister(responseTime)

	// Роутер для prometheus
	r := chi.NewRouter()
	r.Handle("/metrics", promhttp.Handler())

	// Сервер для prometheus
	go func() {
		server := http.Server{Addr: ":80", Handler: r}
		err := server.ListenAndServe()
		if err != nil {
			return
		}
	}()

	return requestsTotal, responseTime
}
