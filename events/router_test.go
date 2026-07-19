package events

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/PastureStack/container-cron/dockerapi"
)

type handlerFunc func(Message)

func (fn handlerFunc) Handle(message Message) {
	fn(message)
}

type closedRouter struct {
	mu    sync.Mutex
	calls int
}

func (router *closedRouter) Listen(context.Context) (<-chan dockerapi.Event, <-chan error) {
	router.mu.Lock()
	router.calls++
	router.mu.Unlock()

	events := make(chan dockerapi.Event)
	errors := make(chan error)
	close(events)
	close(errors)
	return events, errors
}

func (router *closedRouter) callCount() int {
	router.mu.Lock()
	defer router.mu.Unlock()
	return router.calls
}

func TestStartRouterRejectsMissingStreams(t *testing.T) {
	router := routerFunc(func(context.Context) (<-chan dockerapi.Event, <-chan error) {
		return nil, nil
	})
	err := StartRouter(context.Background(), router, handlerFunc(func(Message) {}), 0)
	if err == nil {
		t.Fatal("expected missing stream error")
	}
}

func TestStartRouterWaitsBeforeReconnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	router := &closedRouter{}

	err := StartRouter(ctx, router, handlerFunc(func(Message) {}), 200*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context deadline exceeded", err)
	}
	if calls := router.callCount(); calls != 1 {
		t.Fatalf("Listen calls = %d, want 1 bounded reconnect", calls)
	}
}

type routerFunc func(context.Context) (<-chan dockerapi.Event, <-chan error)

func (fn routerFunc) Listen(ctx context.Context) (<-chan dockerapi.Event, <-chan error) {
	return fn(ctx)
}
