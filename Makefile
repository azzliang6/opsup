.PHONY: init tidy build build-frontend dev test clean reset-data

init tidy:
	go mod tidy

build-frontend:
	cd ui && npm ci && npm run build

build: build-frontend
	CGO_ENABLED=1 go build -trimpath -o opsup .

dev:
	OPSUP_DB_PATH="$${OPSUP_DB_PATH:-$(CURDIR)/dev-data/opsup.db}" go run .

test: build-frontend
	go vet ./...
	go test -race ./...
	cd ui && npm test

clean:
	rm -f opsup opsup-arm64
	rm -rf ui/dist ui/node_modules

reset-data:
	@test "$(CONFIRM)" = "DELETE" || (printf '%s\n' 'Destructive: stop OpsUp, back up your data, then run make reset-data CONFIRM=DELETE DB_PATH=/path/to/opsup.db'; exit 1)
	@test -n "$(DB_PATH)" || (printf '%s\n' 'DB_PATH is required'; exit 1)
	rm -f -- "$(DB_PATH)" "$(DB_PATH)-wal" "$(DB_PATH)-shm"
