# iperf3-Tracker as a container: one Go binary with an embedded web interface.
# Build and start: docker compose up -d --build

# ---------- Build ----------
FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
ARG BUILD_DATE=docker
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X 'iperf3-tracker/internal/version.BuildDate=${BUILD_DATE}'" \
      -o /out/iperf3-tracker ./cmd/iperf3-tracker

# ---------- Runtime ----------
# trixie ships iperf3 3.18; at least 3.17 is required (--json-stream).
# bookworm only has 3.12 and is therefore not an option.
FROM debian:trixie-slim
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      iperf3 traceroute ca-certificates tzdata \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/iperf3-tracker /app/iperf3-tracker
COPY geoip/GeoLite2-City.mmdb /app/geoip/GeoLite2-City.mmdb

# Configuration and database live in the /data volume; config.yaml is created
# with a random token key on first start.
ENV LISTEN_ADDR=0.0.0.0:8000 \
    DB_PATH=/data/iperf3-tracker.db \
    GEOIP_PATH=/app/geoip/GeoLite2-City.mmdb \
    TZ=Europe/Berlin
VOLUME /data
EXPOSE 8000
ENTRYPOINT ["/app/iperf3-tracker", "-config", "/data/config.yaml"]
