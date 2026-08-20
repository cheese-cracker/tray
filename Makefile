BIN := build/tray

.PHONY: build install test flows check fmt vet clean

build:
	go build -o $(BIN) ./cmd/tray

install:
	go install ./cmd/tray

# -timeout is the backstop: a wedged TUI test fails the suite instead of hanging it.
test:
	go test -timeout 120s ./...

flows: build
	./scripts/check-tray.sh

check: fmt vet test flows

fmt:
	gofmt -l -w .

vet:
	go vet ./...

clean:
	rm -rf build
