package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"math"
	"offering/internal/models"
	"time"
)

type Service struct {
	Logger *zap.Logger
	Tracer trace.Tracer
	Config *models.Config
}

func NewService(logger *zap.Logger, tracer trace.Tracer, config *models.Config) *Service {
	return &Service{
		Logger: logger,
		Tracer: tracer,
		Config: config,
	}
}

// hashString возвращает хеш строки
func hashString(input string) int {
	hash := 0
	prime := 23
	mod := 100
	for i := 0; i < len(input); i += 1 {
		hash = (hash + int(input[i])*prime) % mod
	}
	return hash
}

// CreateOffer создает оффер
func (s *Service) CreateOffer(order *models.Order) *models.Order {
	order.Price = models.Price{
		Amount: math.Sqrt(
			(order.From.Lat-order.To.Lat)*(order.From.Lat-order.To.Lat) +
				(order.From.Lng-order.To.Lng)*(order.From.Lng-order.To.Lng)*float64(hashString(order.ClientID)),
		),
		Currency: "RUB",
	}
	return order
}

// JwtOffer превращает order в jwt-токен
func (s *Service) JwtOffer(ctx context.Context, order *models.Order) (string, error) {
	ctx, span := s.Tracer.Start(ctx, "jwt")
	defer span.End()

	// Сериализуем order
	bytes, err := json.Marshal(order)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Marshal error")
		return "", err
	}

	// Устанавливаем claims
	claims := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"order": string(bytes),
		"exp":   time.Now().Add(time.Hour * 8).Unix(), // Срок действия токена: 8 часов
	})

	// Подписываем токен
	token, err := claims.SignedString(&s.Config.PrivateKey)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Signing error")
		return "", err
	}

	return token, nil
}

func (s *Service) UnJwtOffer(ctx context.Context, tokenString string) (*models.Order, error) {
	ctx, span := s.Tracer.Start(ctx, "unjwt")
	defer span.End()

	// Проверка и извлечение данных из токена
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return &s.Config.PrivateKey.PublicKey, nil
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Token read error")
		return nil, err
	}

	// Проверка валидности токена
	if !token.Valid {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Token invalid error")
		return nil, fmt.Errorf("invalid token")
	}

	// Извлечение данных из токена
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Incorrect data in token error")
		return nil, fmt.Errorf("incorrect data in token")
	}
	orderJson, ok := claims["order"].(string)
	if !ok {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Incorrect data in token error")
		return nil, fmt.Errorf("incorrect data in token")
	}

	// Десериализация order
	var order models.Order
	err = json.Unmarshal([]byte(orderJson), &order)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Unmarshal error")
		return nil, err
	}

	return &order, nil
}
