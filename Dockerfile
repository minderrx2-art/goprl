# Go build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app
# Layer dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o main main.go

# Alpine runtime stage
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/main .

EXPOSE 8080
# Run binary
CMD ["./main"]
