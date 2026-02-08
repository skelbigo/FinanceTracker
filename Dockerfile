FROM golang:1.24.4-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux \
    go build -o /app/api ./apps/gateway-http

RUN CGO_ENABLED=0 GOOS=linux \
    go build -o /app/worker ./apps/notification-service/worker

RUN CGO_ENABLED=0 GOOS=linux \
    go build -o /app/notifications-grpc ./apps/notification-service

FROM alpine:3.20

WORKDIR /app

COPY --from=build /app/api /app/api
COPY --from=build /app/worker /app/worker
COPY --from=build /app/notifications-grpc /app/notifications-grpc
COPY migrations /app/migrations
COPY web /app/web

EXPOSE 8080 9090
STOPSIGNAL SIGTERM
ENTRYPOINT ["/app/api"]
