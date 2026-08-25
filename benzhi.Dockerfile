FROM golang:1.22-alpine

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod ./

# Copy all source code
COPY . .

# Build the project to verify compilation
RUN CGO_ENABLED=0 GOOS=linux go build -o /config-center ./cmd/server && echo 'BUILD OK'

# Run go vet to verify static analysis
RUN go vet ./... && echo 'VET OK'

# Expose the default port
EXPOSE 8080

# Start the server
CMD ["/config-center"]