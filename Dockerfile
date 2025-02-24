FROM golang:1.24 AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -ldflags="-s -w" -trimpath -o tellama ./cmd/tellama

FROM gcr.io/distroless/base
COPY --from=builder /app/tellama /app/tellama
WORKDIR /data
ENTRYPOINT ["/app/tellama"]
