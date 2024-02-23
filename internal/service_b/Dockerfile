# Dockerfile for Service B
FROM golang:1.21.1

WORKDIR /app

COPY go.mod go.sum ./
COPY internal/service_b/ .

RUN go mod download
RUN go build -o main_service_b .

CMD ["/app/main_service_b"]