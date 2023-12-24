package main

import (
	"context"
	"fmt"
	"trip/internal/app"
)

func main() {
	ctx := context.Background()

	newApp := app.NewApp(ctx)

	fmt.Println(newApp)
}
