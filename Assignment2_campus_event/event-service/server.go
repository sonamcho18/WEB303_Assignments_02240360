package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "campus-events/gen/eventpb"
)

// eventServer implements pb.EventServiceServer.
//
// Every handler returns a proper gRPC status code so that callers
// (the gateway and the Registration Service) can react correctly:
//
//	InvalidArgument -> bad input, NotFound -> no such event,
//	Internal        -> unexpected database problem.
type eventServer struct {
	pb.UnimplementedEventServiceServer
	store *EventStore
}

func (s *eventServer) CreateEvent(ctx context.Context, req *pb.CreateEventRequest) (*pb.Event, error) {
	if err := validateEventFields(&req.Title, &req.Venue, &req.EventDate, &req.Organizer, req.Capacity); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	ev, err := s.store.Create(ctx, req)
	if err != nil {
		return nil, internalError("create event", err)
	}
	log.Printf("created event id=%d title=%q", ev.Id, ev.Title)
	return ev, nil
}

func (s *eventServer) GetEvent(ctx context.Context, req *pb.GetEventRequest) (*pb.Event, error) {
	if req.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be a positive number")
	}
	ev, err := s.store.Get(ctx, req.Id)
	if errors.Is(err, ErrNotFound) {
		return nil, status.Errorf(codes.NotFound, "event %d not found", req.Id)
	}
	if err != nil {
		return nil, internalError("get event", err)
	}
	return ev, nil
}

func (s *eventServer) ListEvents(ctx context.Context, _ *pb.ListEventsRequest) (*pb.ListEventsResponse, error) {
	events, err := s.store.List(ctx)
	if err != nil {
		return nil, internalError("list events", err)
	}
	return &pb.ListEventsResponse{Events: events}, nil
}

func (s *eventServer) UpdateEvent(ctx context.Context, req *pb.UpdateEventRequest) (*pb.Event, error) {
	if req.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be a positive number")
	}
	if err := validateEventFields(&req.Title, &req.Venue, &req.EventDate, &req.Organizer, req.Capacity); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	ev, err := s.store.Update(ctx, req)
	if errors.Is(err, ErrNotFound) {
		return nil, status.Errorf(codes.NotFound, "event %d not found", req.Id)
	}
	if err != nil {
		return nil, internalError("update event", err)
	}
	log.Printf("updated event id=%d", ev.Id)
	return ev, nil
}

func (s *eventServer) DeleteEvent(ctx context.Context, req *pb.DeleteEventRequest) (*pb.DeleteEventResponse, error) {
	if req.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be a positive number")
	}
	err := s.store.Delete(ctx, req.Id)
	if errors.Is(err, ErrNotFound) {
		return nil, status.Errorf(codes.NotFound, "event %d not found", req.Id)
	}
	if err != nil {
		return nil, internalError("delete event", err)
	}
	log.Printf("deleted event id=%d", req.Id)
	return &pb.DeleteEventResponse{Message: fmt.Sprintf("event %d deleted", req.Id)}, nil
}

// internalError logs the real cause and returns a generic message,
// so database details are never leaked to the client.
func internalError(action string, err error) error {
	log.Printf("ERROR %s: %v", action, err)
	return status.Errorf(codes.Internal, "could not %s, please try again later", action)
}
