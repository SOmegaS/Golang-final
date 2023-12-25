package main

import (
	"context"
	"final-project/internal/app"
	"final-project/internal/logger"
	"github.com/juju/zaputil/zapctx"
	"go.uber.org/zap"
	"log"
	"time"
)

func initProvider(ctx context.Context) func() {
	return func() {
		_, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
	}
}

func main() {
	logger, err := logger.GetLogger(true)
	if err != nil {
		log.Fatal(err)
	}
	ctx := zapctx.WithLogger(context.Background(), logger)
	shutdown := initProvider(ctx)
	defer shutdown()

	logger.Info("init app")
	a, err := app.New(ctx)
	if err != nil {
		logger.Error("init app error", zap.Error(err))
		log.Fatal(err.Error())
	}
	logger.Info("successfully init app")

	logger.Info("running server")
	if err := a.Serve(ctx); err != nil {
		logger.Error("running server error", zap.Error(err))
		log.Fatal(err)
	}
	logger.Info("server stopped")
}
