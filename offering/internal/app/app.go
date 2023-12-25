package app

import (
	"context"
	"encoding/json"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"log"
	"offering/internal/adapter"
	"offering/internal/models"
	"os"
)

const configPath = "./config/config.json"

// App приложение, управляющее главной логикой
type App struct {
	Adapter *adapter.Adapter
	Logger  *zap.Logger
	Tracer  trace.Tracer
	Config  *models.Config
}

func NewApp() *App {
	// Создание логгера
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Logger init error. %v", err)
		return nil
	}
	sugLog := logger.Sugar()
	sugLog.Info("Logger initialized")

	// Инициализация конфига
	sugLog.Info("Initializing config")
	config, err := initConfig()
	if err != nil {
		sugLog.Fatalf("Config init error. %v", err)
		return nil
	}
	sugLog.Info("Config initialized")

	// Инициализация Jaeger
	sugLog.Info("Initializing Jaeger")
	err = initJaeger(config.JaegerAddress)
	if err != nil {
		sugLog.Fatalf("Jaeger init error. %v", err)
		return nil
	}
	sugLog.Info("Jaeger initialized")

	// Создание трейсера
	tracer := otel.Tracer("final")
	sugLog.Info("Tracer created")

	// Создание объекта App
	sugLog.Info("Creating app")
	app := App{
		Adapter: adapter.NewAdapter(logger, tracer, config),
		Logger:  logger,
		Tracer:  tracer,
		Config:  config,
	}
	sugLog.Info("App created")

	return &app
}

// Start начинает работу приложения
func (a *App) Start(ctx context.Context) error {
	a.Logger.Info("Starting app")
	err := a.Adapter.Start(ctx)
	if err != nil {
		a.Logger.Sugar().Fatalf("App error. %v", err)
		return err
	}
	return nil
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
			semconv.ServiceNameKey.String("offering"),
			semconv.DeploymentEnvironmentKey.String("production"),
		)),
	)
	otel.SetTracerProvider(tp)
	return nil
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
