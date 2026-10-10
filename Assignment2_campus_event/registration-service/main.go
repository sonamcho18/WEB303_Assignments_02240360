// Registration Service (Service A)
//
// Owns event registrations and the registration_db database. Before a
// registration is created (and when one is retrieved) it calls the Event
// Service over gRPC, protected by timeout, retry-with-backoff and a
// circuit breaker (see eventclient.go).
package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "campus-events/gen/registrationpb"
	"campus-events/internal/config"
	"campus-events/internal/database"
)

func main() {
	log.SetPrefix("[registration-service] ")
	port := config.MustGet("GRPC_PORT")
	eventAddr := config.MustGet("EVENT_SERVICE_ADDR")

	settings := ResilienceSettings{
		CallTimeout:      config.GetDuration("EVENT_CALL_TIMEOUT", 2*time.Second),
		MaxRetries:       config.GetInt("EVENT_MAX_RETRIES", 3),
		BaseBackoff:      config.GetDuration("EVENT_RETRY_BACKOFF", 200*time.Millisecond),
		FailureThreshold: config.GetInt("CB_FAILURE_THRESHOLD", 3),
		OpenTimeout:      config.GetDuration("CB_OPEN_TIMEOUT", 15*time.Second),
	}
	log.Printf("resilience: timeout=%s retries=%d backoff=%s breaker_threshold=%d breaker_open=%s",
		settings.CallTimeout, settings.MaxRetries, settings.BaseBackoff,
		settings.FailureThreshold, settings.OpenTimeout)

	db, err := database.Connect(config.PostgresDSN())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	store, err := NewRegistrationStore(db)
	if err != nil {
		log.Fatalf("could not create schema: %v", err)
	}

	events, conn, err := NewEventClient(eventAddr, settings)
	if err != nil {
		log.Fatalf("could not create event service client: %v", err)
	}
	defer conn.Close()

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("could not listen on port %s: %v", port, err)
	}

	srv := grpc.NewServer()
	pb.RegisterRegistrationServiceServer(srv, &registrationServer{store: store, events: events})
	reflection.Register(srv)

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop
		log.Println("shutting down")
		srv.GracefulStop()
	}()

	log.Printf("gRPC server listening on :%s (event service at %s)", port, eventAddr)
	if err := srv.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
