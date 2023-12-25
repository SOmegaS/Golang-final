package main

import (
	"context"
	"trip/internal/app"
)

func main() {
	ctx := context.Background()

	newApp := app.NewApp(ctx)

	newApp.Start(ctx)
}
