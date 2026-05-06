.PHONY: init tidy build build-frontend dev clean

init:
	go mod tidy

tidy:
	go mod tidy

build-frontend:
	cd ui && npm install && npm run build

build: build-frontend
	CGO_ENABLED=1 go build -o opsup .

dev:
	go run main.go

clean:
	rm -f opsup opsup.db
	rm -rf ui/dist ui/node_modules
