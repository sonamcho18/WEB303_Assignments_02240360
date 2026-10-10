package main

// eventclient.go — the resilient gRPC client that Service A (Registrations)
// uses to call Service B (Events).
//
// Three resilience patterns are layered around every call:
//
//   1. TIMEOUT          each attempt gets its own deadline (EVENT_CALL_TIMEOUT),
//                       so a slow or frozen Event Service can never hang us.
//   2. RETRY + BACKOFF  transient failures (UNAVAILABLE, DEADLINE_EXCEEDED,
//                       RESOURCE_EXHAUSTED) are retried up to EVENT_MAX_RETRIES
//                       times, waiting 200ms, 400ms, 800ms... plus jitter.
//                       Business errors such as NOT_FOUND are never retried.
//   3. CIRCUIT BREAKER  after CB_FAILURE_THRESHOLD consecutive failed calls the
//                       breaker OPENS and further calls fail immediately for
//                       CB_OPEN_TIMEOUT, giving the Event Service time to
//                       recover. Then one trial call is allowed (HALF-OPEN);
//                       if it succeeds the breaker CLOSES again.
//
//   caller ──► circuit breaker ──► retry loop ──► [timeout] gRPC GetEvent ──► Event Service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"campus-events/gen/eventpb"
)

// ResilienceSettings are loaded from environment variables in main.go.
type ResilienceSettings struct {
	CallTimeout      time.Duration // timeout per attempt
	MaxRetries       int           // retries after the first attempt
	BaseBackoff      time.Duration // first wait between attempts, doubled each time
	FailureThreshold int           // consecutive failures that open the breaker
	OpenTimeout      time.Duration // how long the breaker stays open
}

// EventClient wraps the generated gRPC client with the resilience patterns.
type EventClient struct {
	client   eventpb.EventServiceClient
	breaker  *gobreaker.CircuitBreaker
	settings ResilienceSettings
}

// NewEventClient creates the connection to the Event Service. grpc.NewClient
// connects lazily, so this service still starts if the Event Service is down.
func NewEventClient(addr string, s ResilienceSettings) (*EventClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}

	breaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "event-service",
		MaxRequests: 1,             // trial calls allowed while HALF-OPEN
		Interval:    0,             // never reset counts while CLOSED; a success resets them
		Timeout:     s.OpenTimeout, // OPEN -> HALF-OPEN after this long
		ReadyToTrip: func(c gobreaker.Counts) bool {
			return c.ConsecutiveFailures >= uint32(s.FailureThreshold)
		},
		// A NOT_FOUND or INVALID_ARGUMENT answer means the Event Service is
		// healthy and simply said "no", so it must not count as a failure.
		IsSuccessful: func(err error) bool {
			return err == nil || !isInfrastructureError(status.Code(err))
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Printf("CIRCUIT BREAKER %q: %s -> %s", name, from, to)
		},
	})

	return &EventClient{client: eventpb.NewEventServiceClient(conn), breaker: breaker, settings: s}, conn, nil
}

// GetEvent fetches an event through the circuit breaker and retry loop.
// It always returns a gRPC status error, so the caller can pass it on.
func (c *EventClient) GetEvent(ctx context.Context, id int64) (*eventpb.Event, error) {
	result, err := c.breaker.Execute(func() (interface{}, error) {
		return c.getEventWithRetry(ctx, id)
	})

	switch {
	case errors.Is(err, gobreaker.ErrOpenState), errors.Is(err, gobreaker.ErrTooManyRequests):
		// Fail fast: we do not even try to contact the Event Service.
		return nil, status.Errorf(codes.Unavailable,
			"event service is unavailable (circuit breaker is OPEN); please try again in %s",
			c.settings.OpenTimeout)
	case err != nil:
		return nil, err
	}
	return result.(*eventpb.Event), nil
}

// getEventWithRetry makes up to 1+MaxRetries attempts, each with its own timeout.
func (c *EventClient) getEventWithRetry(ctx context.Context, id int64) (*eventpb.Event, error) {
	attempts := c.settings.MaxRetries + 1
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		// 1. Timeout: a fresh deadline for this attempt only.
		attemptCtx, cancel := context.WithTimeout(ctx, c.settings.CallTimeout)
		ev, err := c.client.GetEvent(attemptCtx, &eventpb.GetEventRequest{Id: id})
		cancel()

		if err == nil {
			if attempt > 1 {
				log.Printf("GetEvent(%d) succeeded on attempt %d/%d", id, attempt, attempts)
			}
			return ev, nil
		}

		code := status.Code(err)
		if !isInfrastructureError(code) {
			return nil, err // e.g. NOT_FOUND: retrying would give the same answer
		}

		lastErr = err
		log.Printf("GetEvent(%d) attempt %d/%d failed: %s (%s)", id, attempt, attempts, code, shortMessage(err))
		if attempt == attempts {
			break
		}

		// 2. Exponential backoff with jitter: 200ms, 400ms, 800ms ... (+0-25%)
		wait := c.settings.BaseBackoff * time.Duration(1<<(attempt-1))
		wait += time.Duration(rand.Int63n(int64(wait)/4 + 1))
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, status.Error(codes.DeadlineExceeded, "request cancelled while waiting to retry the event service")
		}
	}

	if status.Code(lastErr) == codes.DeadlineExceeded {
		return nil, status.Errorf(codes.Unavailable,
			"event service did not respond within %s (%d attempts)", c.settings.CallTimeout, attempts)
	}
	// The low-level cause was logged above; the client gets a clear message.
	return nil, status.Errorf(codes.Unavailable,
		"event service is unavailable (gave up after %d attempts); please try again later", attempts)
}

// isInfrastructureError reports whether a gRPC code means "the Event Service
// could not be reached / did not answer" rather than a normal business reply.
// These are the errors that are retried and that count towards opening the breaker.
func isInfrastructureError(code codes.Code) bool {
	switch code {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted, codes.Internal, codes.Unknown:
		return true
	}
	return false
}

func shortMessage(err error) string {
	if s, ok := status.FromError(err); ok {
		return s.Message()
	}
	return fmt.Sprint(err)
}
