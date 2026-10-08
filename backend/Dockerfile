# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/atish ./cmd/server && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/atish-cli ./cmd/cli

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata wget && adduser -D -u 10001 atish \
    && mkdir -p /data/media && chown -R atish /data
USER atish
COPY --from=build /out/atish /out/atish-cli /usr/local/bin/
ENV MEDIA_DIR=/data/media PORT=8080 APP_ENV=production
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --retries=5 CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["atish"]
