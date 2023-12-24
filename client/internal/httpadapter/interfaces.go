package httpadapter

import (
	"context"
)

type Adapter interface {
	Serve(ctx context.Context) error
	Shutdown(ctx context.Context)
}
