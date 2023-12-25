package adapter

import (
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"io"
	"net/http"
	"offering/internal/models"
	"offering/internal/service"
)

type Adapter struct {
	server  *http.Server
	service *service.Service
	Logger  *zap.Logger
	Tracer  trace.Tracer
}

func NewAdapter(logger *zap.Logger, tracer trace.Tracer, config *models.Config) *Adapter {
	logger.Info("Creating adapter")

	// Создание адаптера
	adapter := Adapter{
		server:  nil, // будет заполнен ниже
		service: service.NewService(logger, tracer, config),
		Logger:  logger,
		Tracer:  tracer,
	}

	// Создание роутера и set путей
	router := chi.NewRouter()
	router.Post("/offers", adapter.createOffer)
	router.Get("/offers/{offerID}", adapter.getOffer)

	// Заполнение сервера с созданным роутером
	adapter.server = &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	logger.Info("Adapter created")

	return &adapter
}

// createOffer обрабатывает запрос на создание заказа, возвращает созданный заказ
func (a *Adapter) createOffer(w http.ResponseWriter, r *http.Request) {
	a.Logger.Info("Creating offer")

	// Старт span-а трейсера
	ctx, span := a.Tracer.Start(r.Context(), "createOffer")
	defer span.End()

	// Считывание bytes из Body
	bytes, err := io.ReadAll(r.Body)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Body reading error")
		w.WriteHeader(http.StatusInternalServerError)
		a.Logger.Sugar().Errorf("Body reading error. %v", err)
		return
	}

	// Десериализация bytes в Order
	order := &models.Order{}
	err = json.Unmarshal(bytes, order)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Unmarshal body to order error")
		w.WriteHeader(http.StatusBadRequest)
		a.Logger.Sugar().Errorf("Unmarshal body to order error. %v", err)
		return
	}

	// Создание заказа
	order = a.service.CreateOffer(order)

	// Создание jwt-токена
	jwtOffer, err := a.service.JwtOffer(ctx, order)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "JWT order error")
		w.WriteHeader(http.StatusInternalServerError)
		a.Logger.Sugar().Errorf("JWT order error. %v", err)
		return
	}

	// Запись ответа
	_, err = w.Write([]byte(jwtOffer))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Writing response error")
		w.WriteHeader(http.StatusInternalServerError)
		a.Logger.Sugar().Errorf("Writing response error. %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)

	a.Logger.Info("Order created")
}

// getOffer возвращает заказ при его наличии
func (a *Adapter) getOffer(w http.ResponseWriter, r *http.Request) {
	a.Logger.Info("Getting offer")

	// Старт span-а трейсера
	ctx, span := a.Tracer.Start(r.Context(), "getOffer")
	defer span.End()

	// Чтение параметра из URL
	offerID := chi.URLParam(r, "offerID")

	// Извлечение информации из JWT-токена
	order, err := a.service.UnJwtOffer(ctx, offerID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Order unjwt error")
		w.WriteHeader(http.StatusBadRequest)
		a.Logger.Sugar().Errorf("Order unjwt error. %v", err)
		return
	}

	// Сериализация Order в bytes
	bytes, err := json.Marshal(order)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Order marshal error")
		w.WriteHeader(http.StatusInternalServerError)
		a.Logger.Sugar().Errorf("Order marshal error. %v", err)
		return
	}

	// Запись ответа
	_, err = w.Write(bytes)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Writing response error")
		w.WriteHeader(http.StatusInternalServerError)
		a.Logger.Sugar().Errorf("Writing response error. %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)

	a.Logger.Info("Offer got")
}

// Start запускает сервер
func (a *Adapter) Start(ctx context.Context) error {
	a.Logger.Info("Starting adapter")

	// Канал, сообщающий о завершении работы сервера
	finish := make(chan error)

	// Запуск сервера
	go func() {
		err := a.server.ListenAndServe()
		finish <- err
	}()

	// Поддержка завершения с контекстом
	var err error
	select {
	case err = <-finish:
		a.Logger.Info("Adapter stopped")
	case <-ctx.Done():
		err = nil
		a.Logger.Info("Adapter stopped because ctx")
	}

	if err != nil {
		a.Logger.Sugar().Errorf("Server error. %v", err)
	}

	return err
}
