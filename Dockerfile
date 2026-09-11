FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /build/ui
COPY ui/package.json ui/package-lock.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend
RUN apk add --no-cache gcc musl-dev
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY main.go .
COPY internal/ internal/
COPY --from=frontend /build/ui/dist/ ui/dist/
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o opsup .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend /build/opsup .
VOLUME /data
ENV OPSUP_DB_PATH=/data/opsup.db
ENV OPSUP_LISTEN=:8080
ENV GIN_MODE=release
EXPOSE 8080
ENTRYPOINT ["./opsup"]
