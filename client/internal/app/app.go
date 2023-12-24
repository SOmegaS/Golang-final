package app

import (
	"context"
	"errors"
	"final-project/internal/httpadapter"
	"github.com/juju/zaputil/zapctx"
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
	httpAdapter httpadapter.Adapter
	client      *mongo.Client
}

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
func initMongo() (*mongo.Client, error) {
	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Connected to MongoDB")
	return client, nil
}

func (a *app) DisconnectMongo() {
	if a.client != nil {
		if err := a.client.Disconnect(context.Background()); err != nil {
			log.Println("Error disconnecting MongoDB:", err)
		}
	}
}

func New(ctx context.Context, config *Config) (App, error) {
	logger := zapctx.Logger(ctx)
	client, err := initMongo()
	if err != nil {
		logger.Error("Init app error", zap.Error(err))
		log.Fatal(err)
	}
	a := &app{
		httpAdapter: httpadapter.New(ctx, &config.HTTP, client),
		client:      client,
	}

	return a, nil
}
