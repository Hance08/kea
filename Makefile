build:
	go build ./cmd/kea

run:
	go run ./cmd/kea

test-race:
	go test -race ./...

spa-install:
	cd spa && npm install

spa-dev:
	cd spa && npm run dev

spa-build:
	cd spa && npm run build
