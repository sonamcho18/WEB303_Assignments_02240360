// API Gateway
//
// The single REST entry point of the system. Postman (or any HTTP client)
// talks only to the gateway; the gateway forwards each request to the
// Event Service or the Registration Service over gRPC and maps gRPC
// status codes to HTTP status codes (see errors.go).
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"campus-events/gen/eventpb"
	"campus-events/gen/registrationpb"
	"campus-events/internal/config"
)

func main() {
	log.SetPrefix("[gateway] ")
	port := config.MustGet("HTTP_PORT")
	eventAddr := config.MustGet("EVENT_SERVICE_ADDR")
	registrationAddr := config.MustGet("REGISTRATION_SERVICE_ADDR")
	timeout := config.GetDuration("REQUEST_TIMEOUT", 15*time.Second)

	eventConn, err := grpc.NewClient(eventAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("event service client: %v", err)
	}
	defer eventConn.Close()

	registrationConn, err := grpc.NewClient(registrationAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("registration service client: %v", err)
	}
	defer registrationConn.Close()

	g := &gateway{
		events:        eventpb.NewEventServiceClient(eventConn),
		registrations: registrationpb.NewRegistrationServiceClient(registrationConn),
		timeout:       timeout,
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           logRequests(g.routes()),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop
		log.Println("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	log.Printf("REST API listening on :%s (events -> %s, registrations -> %s)", port, eventAddr, registrationAddr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

// statusRecorder remembers the status code so it can be logged.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// logRequests prints one line per request, e.g. "POST /events -> 201 (12ms)".
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.RequestURI(), rec.status, time.Since(start).Round(time.Millisecond))
	})
}
