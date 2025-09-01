# build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /surisink ./cmd/surisink

# runtime
FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /surisink /usr/local/bin/surisink
COPY configs/config.example.yaml /app/config.yaml
ENV CONFIG_PATH=/app/config.yaml
ENTRYPOINT ["/usr/local/bin/surisink","--config","/app/config.yaml"]
