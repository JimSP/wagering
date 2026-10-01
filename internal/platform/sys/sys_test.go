package sys

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type brokenRandom struct{}

func (brokenRandom) Read([]byte) (int, error) { return 0, errors.New("entropy unavailable") }
func TestUUIDFailureIsNotSilentlyAccepted(t *testing.T) {
	uuid.SetRand(brokenRandom{})
	defer uuid.SetRand(nil)
	defer func() {
		v := recover()
		e, ok := v.(error)
		if !ok || e.Error() != "entropy unavailable" {
			t.Fatalf("panic=%v", v)
		}
	}()
	UUIDv7{}.NewID()
	t.Fatal("expected panic")
}

func TestClockAndUUID(t *testing.T) {
	before := time.Now()
	now := (Clock{}).Now()
	after := time.Now()
	if now.Before(before) || now.After(after) || now.Location() != time.UTC {
		t.Fatal(now)
	}
	first := (UUIDv7{}).NewID()
	second := (UUIDv7{}).NewID()
	id, e := uuid.Parse(first)
	if e != nil || id.Version() != 7 || first == second {
		t.Fatal(first, second, e)
	}
}
