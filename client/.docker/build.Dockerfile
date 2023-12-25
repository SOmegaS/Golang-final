FROM golang:1.21.4-alpine

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go build -C ./cmd/client-server -o app

CMD ["./cmd/client-server/app"]