package app

import "context"

type App interface {
	Serve(ctx context.Context) error
	Shutdown()
}
