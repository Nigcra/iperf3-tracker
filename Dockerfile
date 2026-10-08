# iperf3-Tracker als Container: eine Go-Binary mit eingebetteter Oberfläche.
# Bauen und starten: docker compose up -d --build

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

# ---------- Laufzeit ----------
# trixie liefert iperf3 3.18; benötigt wird mindestens 3.17 (--json-stream).
# bookworm hat nur 3.12 und scheidet deshalb aus.
FROM debian:trixie-slim
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      iperf3 traceroute ca-certificates tzdata \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/iperf3-tracker /app/iperf3-tracker
COPY geoip/GeoLite2-City.mmdb /app/geoip/GeoLite2-City.mmdb

# Konfiguration und Datenbank liegen im Volume /data; config.yaml wird beim
# ersten Start mit zufälligem Token-Schlüssel angelegt.
ENV LISTEN_ADDR=0.0.0.0:8000 \
    DB_PATH=/data/iperf3-tracker.db \
    GEOIP_PATH=/app/geoip/GeoLite2-City.mmdb \
    TZ=Europe/Berlin
VOLUME /data
EXPOSE 8000
ENTRYPOINT ["/app/iperf3-tracker", "-config", "/data/config.yaml"]
