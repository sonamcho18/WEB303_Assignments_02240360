#!/usr/bin/env sh
# Regenerates the Go code in gen/ from the .proto files.
# The generated code is already committed, so you only need this
# if you change a .proto file.
#
# Requires: protoc, protoc-gen-go, protoc-gen-go-grpc
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.34.2
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
set -e
cd "$(dirname "$0")/.."

mkdir -p gen/eventpb gen/registrationpb

protoc -I proto \
  --go_out=gen/eventpb --go_opt=paths=source_relative \
  --go-grpc_out=gen/eventpb --go-grpc_opt=paths=source_relative \
  event/event.proto

protoc -I proto \
  --go_out=gen/registrationpb --go_opt=paths=source_relative \
  --go-grpc_out=gen/registrationpb --go-grpc_opt=paths=source_relative \
  registration/registration.proto

# source_relative keeps the event/ and registration/ sub-folders; flatten them.
mv gen/eventpb/event/*.go gen/eventpb/ && rmdir gen/eventpb/event
mv gen/registrationpb/registration/*.go gen/registrationpb/ && rmdir gen/registrationpb/registration

echo "Generated code written to gen/"
