package sys

import (
	"time"

	"github.com/google/uuid"
	"go.uber.org/fx"

	"github.com/alexandre/wagering/internal/app/port"
)

type Clock struct{}

func (Clock) Now() time.Time { return time.Now().UTC() }

type UUIDv7 struct{}

func (UUIDv7) NewID() string {
	id, err := uuid.NewV7()
	if err != nil {
		panic(err) // only if the system RNG fails
	}
	return id.String()
}

var Module = fx.Module("sys", fx.Provide(
	fx.Annotate(func() Clock { return Clock{} }, fx.As(new(port.Clock))),
	fx.Annotate(func() UUIDv7 { return UUIDv7{} }, fx.As(new(port.IDGenerator))),
))
