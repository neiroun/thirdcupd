APP := thirdcupd
CONFIG := configs/thirdcupd.example.json

.PHONY: build test run once fmt clean

build:
	go build -trimpath -o bin/$(APP) ./cmd/$(APP)

test:
	go test ./...

run:
	go run ./cmd/$(APP) -config $(CONFIG)

once:
	go run ./cmd/$(APP) -config $(CONFIG) -once

fmt:
	gofmt -w cmd internal

clean:
	rm -rf bin coverage.out
