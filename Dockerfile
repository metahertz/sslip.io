# syntax=docker/dockerfile:1
#
# nsrecord.net DNS server + stats dashboard.
#
# A single Go binary serves both the DNS server (UDP/TCP :53) and the built-in
# stats dashboard (HTTP). The dashboard HTML is compiled into the binary via
# go:embed, so the runtime image only needs the binary itself. A GeoLite2
# country database is baked in at build time so per-country stats work out of
# the box. Settings are supplied via environment variables / an env file
# (see example.env and docker-entrypoint.sh).
#
# Build (BuildKit sets TARGETARCH automatically):
#
#   docker build -t nsrecord-dns-server .
#
# Build with the OFFICIAL MaxMind source instead of the public mirror:
#
#   docker build --build-arg MAXMIND_LICENSE_KEY=xxxxxxxx -t nsrecord-dns-server .
#
# Multi-arch build & push (amd64 + arm64, e.g. for AWS Graviton):
#
#   docker buildx build --platform linux/amd64,linux/arm64 -t youruser/nsrecord-dns-server --push .
#
# Run (DNS on host :53, dashboard on host :8080), settings from an env file:
#
#   docker run -d --rm -p 53:53/udp -p 53:53/tcp -p 8080:8080 \
#     --env-file example.env nsrecord-dns-server
#   dig +short 127.0.0.1.example.com @localhost   # -> 127.0.0.1
#   open http://localhost:8080                     # the dashboard

# ---- build stage ----
FROM golang:1.25 AS build
WORKDIR /src

# Download modules first so this layer is cached unless go.mod/go.sum change.
COPY go.mod go.sum ./
RUN go mod download

# Build a fully static binary (CGO disabled) so it runs on a minimal base image.
# The geoip2/maxminddb libraries are pure Go, so no C toolchain is needed.
COPY . .
ARG VERSION=dev
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -trimpath \
      -ldflags="-s -w -X xip/xip.VersionSemantic=${VERSION}" \
      -o /out/nsrecord-dns-server .

# ---- geoip stage: fetch the GeoLite2-Country database ----
# By default this uses a public mirror (no MaxMind account needed). If you pass
# MAXMIND_LICENSE_KEY, it downloads the official database instead. NOTE: GeoLite2
# is MaxMind's property under their GeoLite2 EULA; the official source (license
# key) is the compliant option for redistribution-sensitive use.
FROM alpine:3.20 AS geoip
ARG GEOIP_MMDB_URL=https://raw.githubusercontent.com/P3TERX/GeoLite.mmdb/download/GeoLite2-Country.mmdb
ARG MAXMIND_LICENSE_KEY=
RUN apk add --no-cache curl tar
RUN set -eux; \
    mkdir -p /geoip; \
    if [ -n "$MAXMIND_LICENSE_KEY" ]; then \
      curl -fSL "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-Country&license_key=${MAXMIND_LICENSE_KEY}&suffix=tar.gz" -o /tmp/geoip.tar.gz; \
      tar -xzf /tmp/geoip.tar.gz -C /tmp; \
      cp /tmp/GeoLite2-Country_*/GeoLite2-Country.mmdb /geoip/GeoLite2-Country.mmdb; \
    else \
      curl -fSL "$GEOIP_MMDB_URL" -o /geoip/GeoLite2-Country.mmdb; \
    fi; \
    test -s /geoip/GeoLite2-Country.mmdb

# ---- runtime stage ----
FROM alpine:3.20

LABEL org.opencontainers.image.title="nsrecord.net DNS server + dashboard"
LABEL org.opencontainers.image.description="sslip.io-style DNS server with embedded-IP NS delegation and a built-in usage dashboard"

# ca-certificates: needed to fetch the blocklist over HTTPS at startup.
# bind-tools: provides `dig`/`nslookup` for in-container troubleshooting.
RUN apk add --no-cache ca-certificates bind-tools

COPY --from=build /out/nsrecord-dns-server /usr/local/bin/nsrecord-dns-server
COPY --from=geoip /geoip/GeoLite2-Country.mmdb /geoip/GeoLite2-Country.mmdb
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh; \
    mkdir -p /data /config

# DNS listens on 53 (UDP mandatory, TCP optional); dashboard defaults to 8080.
# EXPOSE is documentation only; publish with -p at `docker run`.
EXPOSE 53/udp 53/tcp 8080/tcp

# The entrypoint builds flags from env vars / a mounted env file, then execs the
# server. Provide settings via --env-file, compose env_file:, or by mounting an
# env file at /config/nsrecord.env.
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
