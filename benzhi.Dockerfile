FROM golang:1.22

WORKDIR /app

# Copy go module files first for better caching
COPY go.mod ./

# Copy all source code
COPY . .

# Download dependencies (only standard library used, so this is a no-op)
RUN go mod download

# Build the project with CGO disabled for cross-platform compatibility
RUN CGO_ENABLED=0 go build ./...

# Also compile the main binary for direct execution
RUN CGO_ENABLED=0 go build -o /app/server ./cmd/server

# Expose the default port
EXPOSE 8080

# Start the server
CMD ["/app/server"]
