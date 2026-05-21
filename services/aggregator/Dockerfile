FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o aggregator ./cmd/

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/aggregator .
CMD ["./aggregator"]
