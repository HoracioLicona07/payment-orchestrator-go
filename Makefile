.PHONY: run test seed build

run:
	@mkdir -p data
	go run ./cmd/main.go

build:
	CGO_ENABLED=1 go build -o ./bin/orchestrator ./cmd/main.go

test:
	go test ./tests/... -v -count=1

seed:
	@bash scripts/seed.sh

tidy:
	go mod tidy
