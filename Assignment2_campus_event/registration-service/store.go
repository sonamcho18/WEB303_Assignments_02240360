package main

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"

	pb "campus-events/gen/registrationpb"
)

var (
	ErrNotFound      = errors.New("registration not found")
	ErrAlreadyExists = errors.New("student already registered for this event")
	ErrEventFull     = errors.New("event is full")
)

// schema lives only in registration_db.
//
// Note: event_id is NOT a foreign key. The events table is in a different
// database owned by the Event Service (Database per Service), so the link
// is checked through the Event Service's gRPC API instead.
const schema = `
CREATE TABLE IF NOT EXISTS registrations (
    id            BIGSERIAL PRIMARY KEY,
    event_id      BIGINT       NOT NULL,
    student_name  VARCHAR(100) NOT NULL,
    student_id    CHAR(8)      NOT NULL,
    email         VARCHAR(254) NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT one_registration_per_student UNIQUE (event_id, student_id)
);
CREATE INDEX IF NOT EXISTS idx_registrations_event_id ON registrations (event_id);`

type RegistrationStore struct {
	db *sql.DB
}

func NewRegistrationStore(db *sql.DB) (*RegistrationStore, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	return &RegistrationStore{db: db}, nil
}

const selectColumns = `id, event_id, student_name, student_id, email, created_at`

func scanRegistration(row interface{ Scan(...any) error }) (*pb.Registration, error) {
	var (
		r         pb.Registration
		createdAt time.Time
	)
	if err := row.Scan(&r.Id, &r.EventId, &r.StudentName, &r.StudentId, &r.Email, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	r.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return &r, nil
}

// Create inserts a registration if the event still has a free place.
// The advisory lock serialises registrations for the same event, so two
// students cannot both take the last place at the same moment.
func (s *RegistrationStore) Create(ctx context.Context, in *pb.CreateRegistrationRequest, capacity int32) (*pb.Registration, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() // no-op after Commit

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, in.EventId); err != nil {
		return nil, err
	}

	var taken int32
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM registrations WHERE event_id = $1`, in.EventId).Scan(&taken); err != nil {
		return nil, err
	}
	if taken >= capacity {
		return nil, ErrEventFull
	}

	row := tx.QueryRowContext(ctx,
		`INSERT INTO registrations (event_id, student_name, student_id, email)
		 VALUES ($1, $2, $3, $4)
		 RETURNING `+selectColumns,
		in.EventId, in.StudentName, in.StudentId, in.Email)
	r, err := scanRegistration(row)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" { // unique_violation
			return nil, ErrAlreadyExists
		}
		return nil, err
	}
	return r, tx.Commit()
}

func (s *RegistrationStore) Get(ctx context.Context, id int64) (*pb.Registration, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+selectColumns+` FROM registrations WHERE id = $1`, id)
	return scanRegistration(row)
}

// List returns all registrations, or only those for one event when eventID > 0.
func (s *RegistrationStore) List(ctx context.Context, eventID int64) ([]*pb.Registration, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+selectColumns+` FROM registrations
		  WHERE ($1 = 0 OR event_id = $1)
		  ORDER BY id`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []*pb.Registration{}
	for rows.Next() {
		r, err := scanRegistration(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, rows.Err()
}

func (s *RegistrationStore) Update(ctx context.Context, in *pb.UpdateRegistrationRequest) (*pb.Registration, error) {
	row := s.db.QueryRowContext(ctx,
		`UPDATE registrations SET student_name = $2, email = $3
		  WHERE id = $1
		 RETURNING `+selectColumns,
		in.Id, in.StudentName, in.Email)
	return scanRegistration(row)
}

func (s *RegistrationStore) Delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM registrations WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
