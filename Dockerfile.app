FROM golang:1.25.8-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app ./cmd/api

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/app .
COPY --from=builder /app/configs/example.yaml ./configs/example.yaml  

EXPOSE 8080

CMD ["./app"]