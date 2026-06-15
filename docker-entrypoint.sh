#!/bin/sh
#
# Translates environment variables into nsrecord-dns-server flags, then execs
# the server. Settings can come from:
#   - `docker run --env-file foo.env` / compose `env_file:` / `environment:`
#   - an env file mounted into the container (default: /config/nsrecord.env),
#     overridable with ENV_FILE=/path
#
# See example.env for the full list of variables. Flags passed on the command
# line (compose `command:` / extra `docker run` args) are kept and appended.
set -eu

# Source a mounted env file if present (the "mount an env file in" workflow).
ENV_FILE="${ENV_FILE:-/config/nsrecord.env}"
if [ -f "$ENV_FILE" ]; then
  echo "entrypoint: loading settings from $ENV_FILE"
  set -a
  # shellcheck disable=SC1090
  . "$ENV_FILE"
  set +a
fi

# Container defaults (empty string disables the corresponding flag).
: "${PORT:=}"
: "${DASHBOARD:=:8080}"
: "${GEOIP:=/geoip/GeoLite2-Country.mmdb}"
: "${STATS_FILE:=/data/stats.json}"
: "${NAMESERVERS:=}"
: "${ADDRESSES:=}"
: "${DELEGATES:=}"
: "${PTR_DOMAIN:=}"
: "${BLOCKLIST_URL:=}"
: "${PUBLIC:=}"
: "${QUIET:=}"

# Append env-derived flags to any flags already passed on the command line.
# (Flag order is irrelevant to Go's flag package, and the server takes no
# positional args.)
if [ -n "$PORT" ];          then set -- "$@" -port "$PORT"; fi
if [ -n "$DASHBOARD" ];     then set -- "$@" -dashboard "$DASHBOARD"; fi
if [ -n "$STATS_FILE" ];    then set -- "$@" -stats-file "$STATS_FILE"; fi
if [ -n "$NAMESERVERS" ];   then set -- "$@" -nameservers "$NAMESERVERS"; fi
if [ -n "$ADDRESSES" ];     then set -- "$@" -addresses "$ADDRESSES"; fi
if [ -n "$DELEGATES" ];     then set -- "$@" -delegates "$DELEGATES"; fi
if [ -n "$PTR_DOMAIN" ];    then set -- "$@" -ptr-domain "$PTR_DOMAIN"; fi
if [ -n "$BLOCKLIST_URL" ]; then set -- "$@" -blocklistURL "$BLOCKLIST_URL"; fi
if [ "$QUIET" = "true" ];   then set -- "$@" -quiet; fi

# -public is a boolean flag; only pass it when explicitly set.
case "$PUBLIC" in
  true)  set -- "$@" -public=true ;;
  false) set -- "$@" -public=false ;;
esac

# Enable GeoIP only if the database file actually exists, so a missing/disabled
# DB never blocks startup (the server also degrades gracefully on its own).
if [ -n "$GEOIP" ] && [ -f "$GEOIP" ]; then
  set -- "$@" -geoip "$GEOIP"
elif [ -n "$GEOIP" ]; then
  echo "entrypoint: GeoIP DB not found at '$GEOIP'; country stats disabled" >&2
fi

echo "entrypoint: exec nsrecord-dns-server $*"
exec /usr/local/bin/nsrecord-dns-server "$@"
