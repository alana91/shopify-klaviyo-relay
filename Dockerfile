FROM golang:1.26 AS base
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

FROM base AS tester
RUN go test -race ./...

FROM base AS builder
RUN CGO_ENABLED=0 go build -o relay .

FROM alpine:latest AS runner
WORKDIR /app
COPY --from=builder /app/relay .
EXPOSE 8080
CMD ["./relay"]
