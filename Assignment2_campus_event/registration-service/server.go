package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"campus-events/gen/eventpb"
	pb "campus-events/gen/registrationpb"
)

// registrationServer implements pb.RegistrationServiceServer.
type registrationServer struct {
	pb.UnimplementedRegistrationServiceServer
	store  *RegistrationStore
	events *EventClient // the ONLY way this service learns about events
}

// CreateRegistration must call the Event Service before storing anything:
// the event has to exist, and it must still have a free place.
func (s *registrationServer) CreateRegistration(ctx context.Context, req *pb.CreateRegistrationRequest) (*pb.Registration, error) {
	if err := validateCreate(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// gRPC call to Service B, protected by timeout + retry + circuit breaker.
	ev, err := s.events.GetEvent(ctx, req.EventId)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, status.Errorf(codes.NotFound,
				"cannot register: event %d does not exist", req.EventId)
		}
		// UNAVAILABLE etc.: refuse rather than store an unchecked registration.
		log.Printf("create registration rejected, event check failed: %v", err)
		return nil, err
	}

	reg, err := s.store.Create(ctx, req, ev.Capacity)
	switch {
	case errors.Is(err, ErrEventFull):
		return nil, status.Errorf(codes.FailedPrecondition,
			"cannot register: event %d is full (capacity %d)", ev.Id, ev.Capacity)
	case errors.Is(err, ErrAlreadyExists):
		return nil, status.Errorf(codes.AlreadyExists,
			"student %s is already registered for event %d", req.StudentId, req.EventId)
	case err != nil:
		return nil, internalError("create registration", err)
	}

	reg.Event = summary(ev)
	log.Printf("created registration id=%d event_id=%d student_id=%s", reg.Id, reg.EventId, reg.StudentId)
	return reg, nil
}

// GetRegistration returns the registration enriched with live event details.
// If the Event Service is down we still answer (graceful degradation /
// fallback): the registration data is ours, only the event details are missing.
func (s *registrationServer) GetRegistration(ctx context.Context, req *pb.GetRegistrationRequest) (*pb.Registration, error) {
	if req.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be a positive number")
	}
	reg, err := s.store.Get(ctx, req.Id)
	if errors.Is(err, ErrNotFound) {
		return nil, status.Errorf(codes.NotFound, "registration %d not found", req.Id)
	}
	if err != nil {
		return nil, internalError("get registration", err)
	}

	ev, err := s.events.GetEvent(ctx, reg.EventId)
	switch {
	case err == nil:
		reg.Event = summary(ev)
	case status.Code(err) == codes.NotFound:
		reg.EventLookupError = fmt.Sprintf("event %d no longer exists", reg.EventId)
	default:
		log.Printf("FALLBACK: returning registration %d without event details: %v", reg.Id, err)
		reg.EventLookupError = "event details are temporarily unavailable: " + status.Convert(err).Message()
	}
	return reg, nil
}

func (s *registrationServer) ListRegistrations(ctx context.Context, req *pb.ListRegistrationsRequest) (*pb.ListRegistrationsResponse, error) {
	if req.EventId < 0 {
		return nil, status.Error(codes.InvalidArgument, "event_id must be a positive number")
	}
	list, err := s.store.List(ctx, req.EventId)
	if err != nil {
		return nil, internalError("list registrations", err)
	}
	return &pb.ListRegistrationsResponse{Registrations: list}, nil
}

func (s *registrationServer) UpdateRegistration(ctx context.Context, req *pb.UpdateRegistrationRequest) (*pb.Registration, error) {
	if err := validateUpdate(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	reg, err := s.store.Update(ctx, req)
	if errors.Is(err, ErrNotFound) {
		return nil, status.Errorf(codes.NotFound, "registration %d not found", req.Id)
	}
	if err != nil {
		return nil, internalError("update registration", err)
	}
	log.Printf("updated registration id=%d", reg.Id)
	return reg, nil
}

func (s *registrationServer) DeleteRegistration(ctx context.Context, req *pb.DeleteRegistrationRequest) (*pb.DeleteRegistrationResponse, error) {
	if req.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be a positive number")
	}
	err := s.store.Delete(ctx, req.Id)
	if errors.Is(err, ErrNotFound) {
		return nil, status.Errorf(codes.NotFound, "registration %d not found", req.Id)
	}
	if err != nil {
		return nil, internalError("delete registration", err)
	}
	log.Printf("deleted registration id=%d", req.Id)
	return &pb.DeleteRegistrationResponse{Message: fmt.Sprintf("registration %d deleted", req.Id)}, nil
}

func summary(ev *eventpb.Event) *pb.EventSummary {
	return &pb.EventSummary{Title: ev.Title, Venue: ev.Venue, EventDate: ev.EventDate}
}

func internalError(action string, err error) error {
	log.Printf("ERROR %s: %v", action, err)
	return status.Errorf(codes.Internal, "could not %s, please try again later", action)
}
