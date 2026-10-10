package main

import (
	"context"
	"database/sql"
	"errors"
	"time"

	pb "campus-events/gen/eventpb"
)

// ErrNotFound is returned when no event has the requested id.
var ErrNotFound = errors.New("event not found")

// schema is applied at start-up. This table lives only in event_db;
// no other service is allowed to read it (Database per Service).
const schema = `
CREATE TABLE IF NOT EXISTS events (
    id          BIGSERIAL PRIMARY KEY,
    title       VARCHAR(100) NOT NULL,
    venue       VARCHAR(100) NOT NULL,
    event_date  DATE         NOT NULL,
    capacity    INTEGER      NOT NULL CHECK (capacity > 0),
    organizer   VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);`

// EventStore contains all SQL for the events table.
type EventStore struct {
	db *sql.DB
}

func NewEventStore(db *sql.DB) (*EventStore, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	return &EventStore{db: db}, nil
}

const selectColumns = `id, title, venue, event_date, capacity, organizer, created_at`

// scanEvent converts one database row into a protobuf Event.
func scanEvent(row interface{ Scan(...any) error }) (*pb.Event, error) {
	var (
		e         pb.Event
		date      time.Time
		createdAt time.Time
	)
	if err := row.Scan(&e.Id, &e.Title, &e.Venue, &date, &e.Capacity, &e.Organizer, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	e.EventDate = date.Format(dateLayout)
	e.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return &e, nil
}

func (s *EventStore) Create(ctx context.Context, in *pb.CreateEventRequest) (*pb.Event, error) {
	row := s.db.QueryRowContext(ctx,
		`INSERT INTO events (title, venue, event_date, capacity, organizer)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+selectColumns,
		in.Title, in.Venue, in.EventDate, in.Capacity, in.Organizer)
	return scanEvent(row)
}

func (s *EventStore) Get(ctx context.Context, id int64) (*pb.Event, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+selectColumns+` FROM events WHERE id = $1`, id)
	return scanEvent(row)
}

func (s *EventStore) List(ctx context.Context) ([]*pb.Event, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+selectColumns+` FROM events ORDER BY event_date, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []*pb.Event{}
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *EventStore) Update(ctx context.Context, in *pb.UpdateEventRequest) (*pb.Event, error) {
	row := s.db.QueryRowContext(ctx,
		`UPDATE events
		    SET title = $2, venue = $3, event_date = $4, capacity = $5, organizer = $6
		  WHERE id = $1
		 RETURNING `+selectColumns,
		in.Id, in.Title, in.Venue, in.EventDate, in.Capacity, in.Organizer)
	return scanEvent(row)
}

func (s *EventStore) Delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM events WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
