FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go module files first for better caching
COPY go.mod ./

# Copy all source code
COPY . .

# Download dependencies
RUN go mod download

# Build the server binary
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

# Final stage
FROM alpine:latest

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /server /app/server

# Expose the default port
EXPOSE 8080

# Start the server
CMD ["/app/server"]
