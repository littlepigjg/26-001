FROM golang:1.22

WORKDIR /app

COPY go.mod ./
COPY . .

RUN go mod download

RUN mkdir -p /app/data

RUN go build -o /app/server ./cmd/server/

EXPOSE 8080

CMD ["/app/server"]