FROM golang:1.22

WORKDIR /app

# Copy go module files first for better caching
COPY go.mod ./

# Copy all source code
COPY . .

# Download dependencies
RUN go mod download

# Build the project with CGO disabled for cross-platform compatibility
RUN CGO_ENABLED=0 go build ./...

# Expose the default port
EXPOSE 8080

# Start the server
CMD ["go", "run", "./cmd/server"]
