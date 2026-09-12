FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/auth-service/main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o auth-service main.go

FROM alpine:3.19

RUN apk --no-cache add ca-certificates
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=builder /app/auth-service /auth-service

USER appuser

EXPOSE 8083

ENTRYPOINT ["./auth-service"]
