FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o deployscope ./cmd/deployscope

FROM alpine:3.19

RUN apk --no-cache add ca-certificates
COPY --from=builder /app/deployscope /usr/local/bin/deployscope

EXPOSE 8080

USER 1001:1001

ENTRYPOINT ["deployscope"]
