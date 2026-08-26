# Build stage
FROM golang:1.22 AS builder

WORKDIR /app

# Copy go module files first for better caching
COPY go.mod ./

# Copy all source code
COPY . .

# Download dependencies and tidy
RUN go mod tidy

# Build the binary
RUN CGO_ENABLED=0 go build -o config-center-server ./cmd/server

# Runtime stage
FROM golang:1.22

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/config-center-server .

# Expose the default port
EXPOSE 8080

# Start the server
CMD ["./config-center-server"]
