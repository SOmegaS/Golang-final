package app

import (
	"context"
	"encoding/json"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"log"
	"os"
	"trip/internal/models"
	kfk "trip/pkg/kafka"
)

const configPath = "./config/config.json"

type App struct {
	ToClientTopic         *kafka.Conn
	ToDriverTopic         *kafka.Conn
	FromClientDriverTopic *kafka.Conn
	Config                *models.Config
	Logger                *zap.Logger
	Tracer                trace.Tracer
}

func NewApp(ctx context.Context) *App {
	// Создание логгера
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Logger init error. %v", err)
		return nil
	}
	sugLog := logger.Sugar()
	sugLog.Info("Logger initialized")

	// Создание трейсера
	tracer := otel.Tracer("final")
	sugLog.Info("Tracer created")

	// Инициализация конфига
	sugLog.Info("Initializing config")
	config, err := initConfig()
	if err != nil {
		sugLog.Fatalf("Config init error. %v", err)
		return nil
	}

	// Подключение к Kafka
	connClient, err := kfk.ConnectKafka(ctx, config.KafkaAddress, "trip-client-topic", 0)
	if err != nil {
		sugLog.Fatalf("Kafka connect error. %v", err)
		return nil
	}
	connDriver, err := kfk.ConnectKafka(ctx, config.KafkaAddress, "trip-driver-topic", 0)
	if err != nil {
		sugLog.Fatalf("Kafka connect error. %v", err)
		return nil
	}
	connClDrv, err := kfk.ConnectKafka(ctx, config.KafkaAddress, "driver-client-trip-topic", 0)
	if err != nil {
		sugLog.Fatalf("Kafka connect error. %v", err)
		return nil
	}

	// Создание объекта App
	sugLog.Info("Creating app")
	app := App{
		ToClientTopic:         connClient,
		ToDriverTopic:         connDriver,
		FromClientDriverTopic: connClDrv,
		Config:                config,
		Logger:                logger,
		Tracer:                tracer,
	}
	sugLog.Info("App created")

	return &app
}

// initConfig инициализирует конфиг
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
