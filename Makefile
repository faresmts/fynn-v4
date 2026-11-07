.PHONY: install build test dev demo
install:
	cd backend && go mod download
	cd frontend && pnpm install --frozen-lockfile
build:
	cd frontend && pnpm run check && pnpm run build
	cd backend && go build -trimpath -o server ./cmd/server
test:
	cd backend && go test ./... && go vet ./...
	cd frontend && pnpm run check
	cd deploy && python3 -m unittest test_snapshot.py
demo: build
	cd backend && MF_DATABASE=../data/demo.db ./server --demo
dev:
	cd backend && go run ./cmd/server
