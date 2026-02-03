FROM golang:1.25-alpine

WORKDIR /app

# Copy go mod/sum from root (assuming we run build context from root, or relative)
# Since we usually build from tests/benchmark, let's keep it self-contained or use relative paths.
# Based on docker-compose context: ./tests/benchmark.
# We need to ensure we have the files. 

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o benchmark runner.go

ENTRYPOINT ["./benchmark"]
