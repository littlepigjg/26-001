FROM golang:1.22

WORKDIR /app

# Copy go module files first for better caching
COPY go.mod ./

# Download dependencies (only standard library used, so this is a no-op)
RUN go mod download

# Copy all source code
COPY . .

# Build the server binary
RUN go build -o config-center-server ./cmd/server/

# Expose the default port
EXPOSE 8080

# Start the server
CMD ["./config-center-server"]
