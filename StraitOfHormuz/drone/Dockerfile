FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY ./drone.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o drone_bin drone.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/drone_bin .
COPY ./data/initialization/drones.json ./data/initialization/drones.json

CMD ["./drone_bin"]