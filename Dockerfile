FROM golang:1.24.4-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux \
    go build -o /app/api ./cmd/api

FROM alpine:3.20

WORKDIR /app

COPY --from=build /app/api /app/api
COPY migrations /app/migrations

EXPOSE 8080
STOPSIGNAL SIGTERM
ENTRYPOINT ["/app/api"]
