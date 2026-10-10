package main

// JSON shapes used by the REST API. Keeping them separate from the
// generated protobuf types gives the REST API clean, stable field names
// and lets us reject unknown fields in request bodies.

import (
	"campus-events/gen/eventpb"
	"campus-events/gen/registrationpb"
)

// ----- Events -----

type eventInput struct {
	Title     string `json:"title"`
	Venue     string `json:"venue"`
	EventDate string `json:"event_date"`
	Capacity  int32  `json:"capacity"`
	Organizer string `json:"organizer"`
}

type eventJSON struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Venue     string `json:"venue"`
	EventDate string `json:"event_date"`
	Capacity  int32  `json:"capacity"`
	Organizer string `json:"organizer"`
	CreatedAt string `json:"created_at"`
}

func toEventJSON(e *eventpb.Event) eventJSON {
	return eventJSON{
		ID: e.Id, Title: e.Title, Venue: e.Venue, EventDate: e.EventDate,
		Capacity: e.Capacity, Organizer: e.Organizer, CreatedAt: e.CreatedAt,
	}
}

// ----- Registrations -----

type registrationCreateInput struct {
	EventID     int64  `json:"event_id"`
	StudentName string `json:"student_name"`
	StudentID   string `json:"student_id"`
	Email       string `json:"email"`
}

type registrationUpdateInput struct {
	StudentName string `json:"student_name"`
	Email       string `json:"email"`
}

type eventSummaryJSON struct {
	Title     string `json:"title"`
	Venue     string `json:"venue"`
	EventDate string `json:"event_date"`
}

type registrationJSON struct {
	ID               int64             `json:"id"`
	EventID          int64             `json:"event_id"`
	StudentName      string            `json:"student_name"`
	StudentID        string            `json:"student_id"`
	Email            string            `json:"email"`
	CreatedAt        string            `json:"created_at"`
	Event            *eventSummaryJSON `json:"event,omitempty"`
	EventLookupError string            `json:"event_lookup_error,omitempty"`
}

func toRegistrationJSON(r *registrationpb.Registration) registrationJSON {
	out := registrationJSON{
		ID: r.Id, EventID: r.EventId, StudentName: r.StudentName, StudentID: r.StudentId,
		Email: r.Email, CreatedAt: r.CreatedAt, EventLookupError: r.EventLookupError,
	}
	if r.Event != nil {
		out.Event = &eventSummaryJSON{Title: r.Event.Title, Venue: r.Event.Venue, EventDate: r.Event.EventDate}
	}
	return out
}
