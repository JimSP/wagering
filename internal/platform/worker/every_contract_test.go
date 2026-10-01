//go:build go1.25

package worker

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

func TestEveryCancellationAndFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		calls, errorsSeen := 0, 0
		failure := errors.New("step failed")
		go func() {
			done <- Every(time.Second, func(c context.Context) error {
				calls++
				deadline, ok := c.Deadline()
				if !ok || time.Until(deadline) != 15*time.Second {
					t.Error("wrong step deadline")
				}
				return failure
			}, func(e error) {
				if !errors.Is(e, failure) {
					t.Error(e)
				}
				errorsSeen++
				cancel()
			})(ctx)
		}()
		time.Sleep(time.Second)
		synctest.Wait()
		cancel()
		synctest.Wait()
		if e := <-done; e != nil {
			t.Fatal(e)
		}
		if calls != 1 || errorsSeen != 1 {
			t.Fatal(calls, errorsSeen)
		}
	})
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		n := 0
		err := Every(time.Second, func(context.Context) error { n++; return nil }, nil)(ctx)
		if err != nil || n != 0 {
			t.Fatal(n, err)
		}
	})
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		calls := 0
		go func() {
			done <- Every(time.Second, func(c context.Context) error { calls++; cancel(); return nil }, nil)(ctx)
		}()
		time.Sleep(time.Second)
		synctest.Wait()
		cancel()
		synctest.Wait()
		if e := <-done; e != nil || calls != 1 {
			t.Fatal(calls, e)
		}
	})
}

func TestEveryReportsOnlyErrorsAndAllowsAbsentCallback(t *testing.T) {
	for _, failure := range []error{nil, errors.New("step failed")} {
		for _, callback := range []bool{false, true} {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan error, 1)
				calls, reports := 0, 0
				var report func(error)
				if callback {
					report = func(err error) {
						reports++
						if !errors.Is(err, failure) || err == nil {
							t.Errorf("unexpected reported error: %v", err)
						}
					}
				}
				go func() {
					done <- Every(time.Second, func(context.Context) error { calls++; return failure }, report)(ctx)
				}()
				time.Sleep(time.Second)
				synctest.Wait()
				cancel()
				synctest.Wait()
				if err := <-done; err != nil {
					t.Fatal(err)
				}
				want := 0
				if callback && failure != nil {
					want = 1
				}
				if calls != 1 || reports != want {
					t.Fatalf("calls=%d reports=%d want=%d", calls, reports, want)
				}
			})
		}
	}
}
