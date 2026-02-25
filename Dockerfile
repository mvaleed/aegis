FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

# Compile a fully static binary with no CGO, stripped of debug symbols.
# -trimpath removes local filesystem paths from the binary for reproducibility.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -o /build/bin/user-service \
    ./cmd/server

# Using distroless for the smallest possible attack surface.
FROM gcr.io/distroless/static:nonroot

WORKDIR /app

COPY --from=builder --chown=nonroot:nonroot /build/bin/user-service ./user-service

COPY --from=builder --chown=nonroot:nonroot /build/migrations ./migrations

USER nonroot:nonroot

EXPOSE 8080 9090

ENTRYPOINT ["./user-service"]
