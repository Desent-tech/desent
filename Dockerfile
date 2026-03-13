FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /server ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ffmpeg

COPY --from=builder /server /server

WORKDIR /app

EXPOSE 8080 1935

ENTRYPOINT ["/server"]
