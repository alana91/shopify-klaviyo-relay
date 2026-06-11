FROM golang:1.26 AS base
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

FROM base AS test
CMD ["go", "test", "-race", "./..."]

FROM base AS builder
RUN CGO_ENABLED=0 go build -o relay .

FROM alpine:3.20 AS runner
RUN adduser -D -u 10001 app
WORKDIR /app
COPY --from=builder /app/relay .
USER app
EXPOSE 8080
CMD ["./relay"]
