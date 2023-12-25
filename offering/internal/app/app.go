package app

import (
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"log"
	"net/http"
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

	// Prometheus
	sugLog.Info("Initializing Prometheus")
	requestsTotal, responseTime := initPrometheus()
	sugLog.Info("Prometheus initialized")

	// Создание объекта App
	sugLog.Info("Creating app")
	app := App{
		Adapter: adapter.NewAdapter(logger, tracer, config, requestsTotal, responseTime),
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
