.PHONY: build test lint fmt tidy docker-up docker-down migrate clean

BINARY := rook
PKG := ./...

build:
	go build -o $(BINARY) ./cmd/rook

test:
	go test -race -count=1 $(PKG)

lint:
	golangci-lint run

fmt:
	gofmt -s -w .
	goimports -w -local github.com/MapleRook/rook .

tidy:
	go mod tidy

docker-up:
	docker compose up -d

docker-down:
	docker compose down

migrate:
	goose -dir ./migrations postgres "$(ROOK_DATABASE_URL)" up

clean:
	rm -f $(BINARY) $(BINARY).exe
