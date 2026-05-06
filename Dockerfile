# ---- 构建阶段 ----
FROM --platform=$BUILDPLATFORM node:20-alpine AS frontend

WORKDIR /build/ui
COPY ui/package.json ui/package-lock.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS backend

ARG TARGETPLATFORM
RUN apk add --no-cache gcc musl-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY main.go .
COPY internal/ internal/
COPY --from=frontend /build/ui/dist/ ui/dist/

RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o opsup .

# ---- 运行阶段 ----
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=backend /build/opsup .

VOLUME /data

ENV OPSUP_DB_PATH=/data/opsup.db
ENV OPSUP_LISTEN=:8080

EXPOSE 8080

ENTRYPOINT ["./opsup"]
