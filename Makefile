.PHONY: build run migrate dev clean

build:
	go build -o bin/anon ./cmd/server

run: build
	./bin/anon

dev:
	docker-compose up --build

migrate:
	docker-compose exec app sh -c "echo 'migrations run on startup'"

clean:
	docker-compose down -v
	rm -rf bin/

test:
	go test ./... -v

tidy:
	go mod tidy

lint:
	golangci-lint run ./...
