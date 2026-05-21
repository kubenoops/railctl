# ── Builder ──────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

WORKDIR /src

# Cache deps
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 go build \
    -ldflags "-X github.com/kubenoops/railctl/internal/cmd.version=${VERSION} \
              -X github.com/kubenoops/railctl/internal/cmd.commit=${COMMIT} \
              -X github.com/kubenoops/railctl/internal/cmd.date=${DATE}" \
    -o /railctl ./cmd/railctl

# ── Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates && \
    adduser -D -h /home/railctl railctl

COPY --from=builder /railctl /usr/local/bin/railctl

USER railctl
ENTRYPOINT ["railctl"]
