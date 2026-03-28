
# Commands for pubsubmsgrestforwarder
default:
  @just --list
# Build pubsubmsgrestforwarder with Go
build:
  go build ./...

# Run tests for pubsubmsgrestforwarder with Go
test:
  go clean -testcache
  go test ./...