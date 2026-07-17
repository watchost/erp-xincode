.PHONY: build run test migrate-up migrate-down lint clean

build:
	CGO_ENABLED=0 go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./... -v

migrate-up:
	migrate -path ./migrations -database "postgres://erp:erp123@localhost:5432/erp_dev?sslmode=disable" up

migrate-down:
	migrate -path ./migrations -database "postgres://erp:erp123@localhost:5432/erp_dev?sslmode=disable" down

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/
