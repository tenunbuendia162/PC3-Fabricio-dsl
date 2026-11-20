.PHONY: test race cover bdd cli web build docker clean

test:   ; go test ./...
race:   ; go test -race ./...
cover:  ; go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1
bdd:    ; go run ./cmd/cli -bdd
cli:    ; go run ./cmd/cli
web:    ; go run ./cmd/web -addr :8080
build:  ; mkdir -p bin && go build -o bin/cajero-cli ./cmd/cli && go build -o bin/cajero-web ./cmd/web
docker: ; docker build -t cajero-web-uni .
clean:  ; rm -rf bin coverage.out data/banco.json
