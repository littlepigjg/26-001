FROM golang:1.22

WORKDIR /app

# Copy all source code
COPY . .

# Initialize module and download dependencies
RUN go mod tidy

# Build the binary
RUN CGO_ENABLED=0 go build -o config-center ./cmd/server

# Expose the default port
EXPOSE 8080

# Start the server
CMD ["./config-center"]
