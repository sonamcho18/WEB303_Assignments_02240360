# Campus Events – WEB303 Assignment

A small microservices system for campus events, written in Go.
Student clubs create **events**, and students **register** for them.

## Services

| Service | Role | Port | Database |
|---|---|---|---|
| **gateway** | REST API entry point (used by Postman) | 8080 | – |
| **registration-service** | Service A – manages registrations, calls the Event Service | 50052 (gRPC) | registration_db |
| **event-service** | Service B – manages events | 50051 (gRPC) | event_db |

![Architecture](docs/architecture.png)

- Postman talks only to the **gateway** (REST).
- The gateway talks to the services using **gRPC**.
- Before creating a registration, the Registration Service checks over gRPC that the event exists and is not full.
- Each service has **its own PostgreSQL database** (Database per Service).

## How to run

Requirements: **Docker Desktop**

```bash
docker compose up --build
```

Wait for `REST API listening on :8080`, then open:

```
http://localhost:8080/health
```

You should see `{"status": "ok"}`.

To stop: press `Ctrl + C`, or run `docker compose down`.
To start with empty databases: `docker compose down -v`.

## API endpoints

| Method | URL | Description |
|---|---|---|
| POST | `/events` | Create an event |
| GET | `/events` | List events |
| GET | `/events/{id}` | Get one event |
| PUT | `/events/{id}` | Update an event |
| DELETE | `/events/{id}` | Delete an event |
| POST | `/registrations` | Register a student (checks the event via gRPC) |
| GET | `/registrations` | List registrations (`?event_id=1` to filter) |
| GET | `/registrations/{id}` | Get one registration with event details |
| PUT | `/registrations/{id}` | Update student name or email |
| DELETE | `/registrations/{id}` | Cancel a registration |

Example request body:

```json
POST /registrations
{
  "event_id": 1,
  "student_name": "Pema Wangmo",
  "student_id": "02240365",
  "email": "pema@rub.edu.bt"
}
```

### Error codes

| gRPC error | HTTP status | Example |
|---|---|---|
| INVALID_ARGUMENT | 400 | Missing or invalid fields |
| NOT_FOUND | 404 | Event does not exist |
| ALREADY_EXISTS | 409 | Student already registered |
| FAILED_PRECONDITION | 409 | Event is full |
| UNAVAILABLE | 503 | Event Service is down |

## Resilience

The call from the Registration Service to the Event Service uses:

- **Timeout:** each attempt waits at most 2 seconds
- **Retry with backoff:** up to 4 attempts (waits 200ms, 400ms, 800ms)
- **Circuit breaker:** opens after 3 failed calls, then rejects requests instantly for 15 seconds

To test it:

```bash
docker compose stop event-service    # simulate a failure
# send POST /registrations from Postman a few times → 503
docker compose logs registration-service
docker compose start event-service   # recover
```

## Configuration

All settings (ports, database details, timeouts) are in the `.env` file
and passed to the containers by `docker-compose.yml`. Nothing is hard-coded.

## Testing with Postman

Import `postman/campus-events.postman_collection.json` and run the folders in order (0 → 6).
Folder 5 needs the Event Service stopped first.

## Project structure

```
proto/                  .proto files (gRPC contracts)
gen/                    generated Go code
event-service/          Service B + Dockerfile
registration-service/   Service A + Dockerfile (resilience in eventclient.go)
gateway/                API Gateway + Dockerfile
internal/               shared config and database helpers
postman/                Postman collection
docs/                   architecture diagram
docker-compose.yml
.env
```
