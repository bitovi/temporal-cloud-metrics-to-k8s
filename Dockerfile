FROM golang:1.22.3

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY src/*.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o ./temporal-cloud-metrics-adapter

CMD ["./temporal-cloud-metrics-adapter"]