package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// grpcToHTTP maps a gRPC status code to the matching HTTP status code.
var grpcToHTTP = map[codes.Code]int{
	codes.OK:                 http.StatusOK,
	codes.InvalidArgument:    http.StatusBadRequest,          // 400
	codes.OutOfRange:         http.StatusBadRequest,          // 400
	codes.Unauthenticated:    http.StatusUnauthorized,        // 401
	codes.PermissionDenied:   http.StatusForbidden,           // 403
	codes.NotFound:           http.StatusNotFound,            // 404
	codes.AlreadyExists:      http.StatusConflict,            // 409
	codes.FailedPrecondition: http.StatusConflict,            // 409 (e.g. event is full)
	codes.Aborted:            http.StatusConflict,            // 409
	codes.ResourceExhausted:  http.StatusTooManyRequests,     // 429
	codes.Canceled:           499,                            // client closed request
	codes.Unimplemented:      http.StatusNotImplemented,      // 501
	codes.Unavailable:        http.StatusServiceUnavailable,  // 503
	codes.DeadlineExceeded:   http.StatusGatewayTimeout,      // 504
	codes.Internal:           http.StatusInternalServerError, // 500
	codes.Unknown:            http.StatusInternalServerError, // 500
	codes.DataLoss:           http.StatusInternalServerError, // 500
}

// errorBody is the JSON shape of every error the gateway returns:
//
//	{"error": {"status": 404, "code": "NOT_FOUND", "message": "event 9 not found"}}
type errorBody struct {
	Error struct {
		Status  int    `json:"status"`
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// writeGRPCError converts an error returned by a gRPC call into an HTTP response.
// service names the backend that was called, for clearer messages.
func writeGRPCError(w http.ResponseWriter, err error, service string) {
	st := status.Convert(err)
	httpStatus, ok := grpcToHTTP[st.Code()]
	if !ok {
		httpStatus = http.StatusInternalServerError
	}

	msg := st.Message()
	// If the gateway itself could not reach the service, gRPC returns a raw
	// transport error. Log it, but give the client a readable message.
	// ("connection refused" locally; "name resolver" / "zero addresses" when a
	// Docker container is stopped and its hostname no longer resolves.)
	if st.Code() == codes.Unavailable && isTransportError(msg) {
		log.Printf("%s unreachable: %s", service, msg)
		msg = service + " is unavailable, please try again later"
	}
	if st.Code() == codes.DeadlineExceeded {
		msg = service + " did not respond in time, please try again later"
	}
	writeError(w, httpStatus, codeName(st.Code()), msg)
}

func isTransportError(msg string) bool {
	for _, s := range []string{"connection error", "name resolver", "zero addresses", "transport", "dial tcp"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// writeError sends a JSON error with the given HTTP status.
func writeError(w http.ResponseWriter, httpStatus int, code, message string) {
	var body errorBody
	body.Error.Status = httpStatus
	body.Error.Code = code
	body.Error.Message = message
	writeJSON(w, httpStatus, body)
}

func writeJSON(w http.ResponseWriter, httpStatus int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Printf("could not write response: %v", err)
	}
}

// codeName turns codes.NotFound into "NOT_FOUND", matching the gRPC spec names.
func codeName(c codes.Code) string {
	names := map[codes.Code]string{
		codes.OK: "OK", codes.Canceled: "CANCELLED", codes.Unknown: "UNKNOWN",
		codes.InvalidArgument: "INVALID_ARGUMENT", codes.DeadlineExceeded: "DEADLINE_EXCEEDED",
		codes.NotFound: "NOT_FOUND", codes.AlreadyExists: "ALREADY_EXISTS",
		codes.PermissionDenied: "PERMISSION_DENIED", codes.ResourceExhausted: "RESOURCE_EXHAUSTED",
		codes.FailedPrecondition: "FAILED_PRECONDITION", codes.Aborted: "ABORTED",
		codes.OutOfRange: "OUT_OF_RANGE", codes.Unimplemented: "UNIMPLEMENTED",
		codes.Internal: "INTERNAL", codes.Unavailable: "UNAVAILABLE",
		codes.DataLoss: "DATA_LOSS", codes.Unauthenticated: "UNAUTHENTICATED",
	}
	if n, ok := names[c]; ok {
		return n
	}
	return c.String()
}
