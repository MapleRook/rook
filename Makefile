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

migrate: build
	./$(BINARY) migrate

clean:
	rm -f $(BINARY) $(BINARY).exe
