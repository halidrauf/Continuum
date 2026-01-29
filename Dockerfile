FROM golang:alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o main ./src

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/main .
COPY .env . 

ENV OTEL_RESOURCE_ATTRIBUTES="service.name=continuum.worker,service.version=0.1.0"

# We need ca-certificates for any external requests (if any), and potentially libc compatibility
RUN apk --no-cache add ca-certificates

CMD ["./main"]
