// Event Service (Service B)
//
// Owns campus events and the event_db database. It exposes full CRUD over
// gRPC and has no knowledge of registrations; other services may only see
// events through this gRPC API.
package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "campus-events/gen/eventpb"
	"campus-events/internal/config"
	"campus-events/internal/database"
)

func main() {
	log.SetPrefix("[event-service] ")
	port := config.MustGet("GRPC_PORT")

	db, err := database.Connect(config.PostgresDSN())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	store, err := NewEventStore(db)
	if err != nil {
		log.Fatalf("could not create schema: %v", err)
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("could not listen on port %s: %v", port, err)
	}

	srv := grpc.NewServer()
	pb.RegisterEventServiceServer(srv, &eventServer{store: store})
	reflection.Register(srv) // lets tools like grpcurl / Postman gRPC discover the API

	// Stop cleanly on Ctrl+C or `docker compose stop`.
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop
		log.Println("shutting down")
		srv.GracefulStop()
	}()

	log.Printf("gRPC server listening on :%s", port)
	if err := srv.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
