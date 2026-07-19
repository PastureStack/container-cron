package events

import (
	"context"
	"fmt"
	"time"

	"github.com/PastureStack/container-cron/dockerapi"
	"github.com/sirupsen/logrus"
)

// Router Interface
type Router interface {
	Listen(context.Context) (<-chan dockerapi.Event, <-chan error)
}

// DockerEventRouter is the Docker event handler implementation
type DockerEventRouter struct {
	DockerClient *dockerapi.Client
	Handler      Handler
}

// NewEventRouter returns the a Docker event handler
func NewEventRouter() (Router, error) {
	dClient, err := dockerapi.NewFromEnv()
	if err != nil {
		return nil, err
	}
	return DockerEventRouter{
		DockerClient: dClient,
	}, nil
}

// StartRouter calls the listener function and takes the interface for testing
func StartRouter(ctx context.Context, router Router, handler Handler, retryDelay time.Duration) error {
	if retryDelay < 0 {
		return fmt.Errorf("retry delay must not be negative")
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		streamCtx, cancel := context.WithCancel(ctx)
		eventStream, errChan := router.Listen(streamCtx)
		if eventStream == nil && errChan == nil {
			cancel()
			return fmt.Errorf("router returned no event or error stream")
		}

	streamLoop:
		for eventStream != nil || errChan != nil {
			select {
			case <-ctx.Done():
				cancel()
				return ctx.Err()
			case event, ok := <-eventStream:
				if !ok {
					eventStream = nil
					continue
				}
				handler.Handle(&event)
			case err, ok := <-errChan:
				if !ok {
					errChan = nil
					continue
				}
				if err != nil {
					logrus.Errorf("Docker event stream failed: %v", err)
				}
				break streamLoop
			}
		}
		cancel()

		timer := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// Listen implements the Router interface
func (de DockerEventRouter) Listen(ctx context.Context) (<-chan dockerapi.Event, <-chan error) {
	return de.DockerClient.Events(ctx, []string{
		"start",
		"create",
		"stop",
		"die",
		"destroy",
	})
}
