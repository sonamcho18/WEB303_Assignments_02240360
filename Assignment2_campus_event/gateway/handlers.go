package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"campus-events/gen/eventpb"
	"campus-events/gen/registrationpb"
)

// gateway translates REST calls into gRPC calls.
type gateway struct {
	events        eventpb.EventServiceClient
	registrations registrationpb.RegistrationServiceClient
	timeout       time.Duration
}

// routes registers the resource-based URLs (Go 1.22+ method patterns).
func (g *gateway) routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /events", g.createEvent)
	mux.HandleFunc("GET /events", g.listEvents)
	mux.HandleFunc("GET /events/{id}", g.getEvent)
	mux.HandleFunc("PUT /events/{id}", g.updateEvent)
	mux.HandleFunc("DELETE /events/{id}", g.deleteEvent)

	mux.HandleFunc("POST /registrations", g.createRegistration)
	mux.HandleFunc("GET /registrations", g.listRegistrations)
	mux.HandleFunc("GET /registrations/{id}", g.getRegistration)
	mux.HandleFunc("PUT /registrations/{id}", g.updateRegistration)
	mux.HandleFunc("DELETE /registrations/{id}", g.deleteRegistration)

	return mux
}

// ---------- Events ----------

func (g *gateway) createEvent(w http.ResponseWriter, r *http.Request) {
	var in eventInput
	if !decodeJSON(w, r, &in) {
		return
	}
	ctx, cancel := g.ctx(r)
	defer cancel()

	ev, err := g.events.CreateEvent(ctx, &eventpb.CreateEventRequest{
		Title: in.Title, Venue: in.Venue, EventDate: in.EventDate,
		Capacity: in.Capacity, Organizer: in.Organizer,
	})
	if err != nil {
		writeGRPCError(w, err, "event service")
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/events/%d", ev.Id))
	writeJSON(w, http.StatusCreated, toEventJSON(ev))
}

func (g *gateway) listEvents(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := g.ctx(r)
	defer cancel()

	resp, err := g.events.ListEvents(ctx, &eventpb.ListEventsRequest{})
	if err != nil {
		writeGRPCError(w, err, "event service")
		return
	}
	out := make([]eventJSON, 0, len(resp.Events))
	for _, e := range resp.Events {
		out = append(out, toEventJSON(e))
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": out, "count": len(out)})
}

func (g *gateway) getEvent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ctx, cancel := g.ctx(r)
	defer cancel()

	ev, err := g.events.GetEvent(ctx, &eventpb.GetEventRequest{Id: id})
	if err != nil {
		writeGRPCError(w, err, "event service")
		return
	}
	writeJSON(w, http.StatusOK, toEventJSON(ev))
}

func (g *gateway) updateEvent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in eventInput
	if !decodeJSON(w, r, &in) {
		return
	}
	ctx, cancel := g.ctx(r)
	defer cancel()

	ev, err := g.events.UpdateEvent(ctx, &eventpb.UpdateEventRequest{
		Id: id, Title: in.Title, Venue: in.Venue, EventDate: in.EventDate,
		Capacity: in.Capacity, Organizer: in.Organizer,
	})
	if err != nil {
		writeGRPCError(w, err, "event service")
		return
	}
	writeJSON(w, http.StatusOK, toEventJSON(ev))
}

func (g *gateway) deleteEvent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ctx, cancel := g.ctx(r)
	defer cancel()

	resp, err := g.events.DeleteEvent(ctx, &eventpb.DeleteEventRequest{Id: id})
	if err != nil {
		writeGRPCError(w, err, "event service")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": resp.Message})
}

// ---------- Registrations ----------

func (g *gateway) createRegistration(w http.ResponseWriter, r *http.Request) {
	var in registrationCreateInput
	if !decodeJSON(w, r, &in) {
		return
	}
	ctx, cancel := g.ctx(r)
	defer cancel()

	reg, err := g.registrations.CreateRegistration(ctx, &registrationpb.CreateRegistrationRequest{
		EventId: in.EventID, StudentName: in.StudentName, StudentId: in.StudentID, Email: in.Email,
	})
	if err != nil {
		writeGRPCError(w, err, "registration service")
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/registrations/%d", reg.Id))
	writeJSON(w, http.StatusCreated, toRegistrationJSON(reg))
}

// listRegistrations supports an optional filter: GET /registrations?event_id=1
func (g *gateway) listRegistrations(w http.ResponseWriter, r *http.Request) {
	var eventID int64
	if v := r.URL.Query().Get("event_id"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "event_id query parameter must be a positive whole number")
			return
		}
		eventID = n
	}
	ctx, cancel := g.ctx(r)
	defer cancel()

	resp, err := g.registrations.ListRegistrations(ctx, &registrationpb.ListRegistrationsRequest{EventId: eventID})
	if err != nil {
		writeGRPCError(w, err, "registration service")
		return
	}
	out := make([]registrationJSON, 0, len(resp.Registrations))
	for _, reg := range resp.Registrations {
		out = append(out, toRegistrationJSON(reg))
	}
	writeJSON(w, http.StatusOK, map[string]any{"registrations": out, "count": len(out)})
}

func (g *gateway) getRegistration(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ctx, cancel := g.ctx(r)
	defer cancel()

	reg, err := g.registrations.GetRegistration(ctx, &registrationpb.GetRegistrationRequest{Id: id})
	if err != nil {
		writeGRPCError(w, err, "registration service")
		return
	}
	writeJSON(w, http.StatusOK, toRegistrationJSON(reg))
}

func (g *gateway) updateRegistration(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in registrationUpdateInput
	if !decodeJSON(w, r, &in) {
		return
	}
	ctx, cancel := g.ctx(r)
	defer cancel()

	reg, err := g.registrations.UpdateRegistration(ctx, &registrationpb.UpdateRegistrationRequest{
		Id: id, StudentName: in.StudentName, Email: in.Email,
	})
	if err != nil {
		writeGRPCError(w, err, "registration service")
		return
	}
	writeJSON(w, http.StatusOK, toRegistrationJSON(reg))
}

func (g *gateway) deleteRegistration(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ctx, cancel := g.ctx(r)
	defer cancel()

	resp, err := g.registrations.DeleteRegistration(ctx, &registrationpb.DeleteRegistrationRequest{Id: id})
	if err != nil {
		writeGRPCError(w, err, "registration service")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": resp.Message})
}

// ---------- helpers ----------

// ctx gives every downstream gRPC call a deadline, so the gateway never waits forever.
func (g *gateway) ctx(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), g.timeout)
}

// pathID reads {id} from the URL and rejects anything that is not a positive integer.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "id in the URL must be a positive whole number")
		return 0, false
	}
	return id, true
}

// decodeJSON parses the request body strictly: it must be valid JSON,
// contain no unknown fields and be a single object. Field-level rules
// (required, formats, ranges) are enforced by the services themselves.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var typeErr *json.UnmarshalTypeError
		msg := "request body must be valid JSON: " + err.Error()
		switch {
		case errors.Is(err, io.EOF):
			msg = "request body is empty; a JSON object is required"
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			msg = "request body contains an " + strings.TrimPrefix(err.Error(), "json: ")
		case errors.As(err, &typeErr):
			kind := "text"
			if typeErr.Type.Kind() != reflect.String {
				kind = "a whole number"
			}
			msg = fmt.Sprintf("field %q must be %s", typeErr.Field, kind)
		}
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", msg)
		return false
	}
	if dec.More() {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "request body must contain a single JSON object")
		return false
	}
	return true
}
