package app

import (
	"context"
	"encoding/json"
	"errors"
	"final-project/internal/httpadapter"
	"final-project/models"
	kfk "final-project/pkg/kafka"
	"github.com/juju/zaputil/zapctx"
	"github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

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

// initJaeger подключает Jaeger для трейсинга
func initJaeger(address string) error {
	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint("http://" + address + "/api/traces")))
	if err != nil {
		return err
	}
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exporter),
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("client"),
			semconv.DeploymentEnvironmentKey.String("production"),
		)),
	)
	otel.SetTracerProvider(tp)
	return nil
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

	// Инициализация Jaeger
	logger.Info("Initializing Jaeger")
	err = initJaeger(config.JaegerAddress)
	if err != nil {
		logger.Error("Jaeger init error. %v", zap.Error(err))
		log.Fatal(err)
	}
	logger.Info("Jaeger initialized")

	// Создание трейсера
	tracer := otel.Tracer("final")
	logger.Info("Tracer created")

	a := &app{
		httpAdapter:   httpadapter.New(ctx, config, tracer, client, connTrip, connDriver),
		client:        client,
		fromTripTopic: connTrip,
		toDriverTopic: connDriver,
	}

	return a, nil
}
