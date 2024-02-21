# Dockerfile for Service A
FROM golang:1.21.1

WORKDIR /app

COPY go.mod go.sum ./
COPY internal/service_a/ .

RUN go mod download
RUN go build -o main_service_a .

CMD ["/app/main_service_a"]