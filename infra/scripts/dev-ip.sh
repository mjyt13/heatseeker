#!/usr/bin/env bash
# Point the local dev config at this machine's Wi-Fi/LAN address, so a phone in
# the same network reaches the API, MinIO and Metro. Run after switching
# networks: `task dev:ip` (or `task dev:ip -- 192.168.1.20` to pick the address).
#
# Updates (creating the lines when missing):
#   apps/api/.env          APP_BASE_URL, S3_PUBLIC_ENDPOINT (host only; ports kept)
#                          and LAN IPs inside CORS_ORIGINS
#   apps/mobile/.env.local REACT_NATIVE_PACKAGER_HOSTNAME (address Expo announces
#                          in exp://…; without it Expo may pick a VPN interface)
#                          and the host of EXPO_PUBLIC_API_URL, if set
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
api_env="$root/apps/api/.env"
mobile_env="$root/apps/mobile/.env.local"

is_private() {
  [[ "$1" =~ ^10\. || "$1" =~ ^192\.168\. || "$1" =~ ^172\.(1[6-9]|2[0-9]|3[01])\. ]]
}

detect_ip() {
  # Physical interfaces (wl*, en*, eth*) first; VPN tunnels, containers and
  # bridges are skipped.
  ip -4 -o addr show scope global |
    awk '{split($4, a, "/"); print $2, a[1]}' |
    grep -Ev '^(lo|tun|tap|wg|singbox|tailscale|zt|docker|podman|cni|br-|virbr|veth)' |
    awk '{print ($1 ~ /^(wl|en|eth)/ ? 0 : 1), $2}' |
    sort -s -n -k1,1 |
    while read -r _ addr; do
      if is_private "$addr"; then
        echo "$addr"
        break
      fi
    done
}

ip_addr="${1:-$(detect_ip)}"
if [[ -z "$ip_addr" ]]; then
  echo "LAN address not found (ip -4 addr). Pass it explicitly: task dev:ip -- 192.168.x.y" >&2
  exit 1
fi
if ! [[ "$ip_addr" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "not an IPv4 address: $ip_addr" >&2
  exit 1
fi

# set_url_host FILE KEY DEFAULT: replace the host of the URL in KEY, keeping the
# scheme, port, path and a trailing comment; add KEY=DEFAULT when missing.
set_url_host() {
  local file="$1" key="$2" default="$3"
  if grep -qE "^$key=" "$file"; then
    sed -i -E "s#^($key=[a-z]+://)[^:/[:space:]]+#\1$ip_addr#" "$file"
  else
    printf '%s=%s\n' "$key" "$default" >>"$file"
  fi
}

# set_value FILE KEY VALUE: replace or add KEY=VALUE.
set_value() {
  local file="$1" key="$2" value="$3"
  if grep -qE "^$key=" "$file"; then
    sed -i -E "s#^$key=.*#$key=$value#" "$file"
  else
    printf '%s=%s\n' "$key" "$value" >>"$file"
  fi
}

ensure_newline() {
  [[ -s "$1" && -n "$(tail -c1 "$1")" ]] && echo >>"$1"
  return 0
}

if [[ ! -f "$api_env" ]]; then
  echo "$api_env is missing: cp .env.example apps/api/.env (README, «Запуск»)" >&2
  exit 1
fi
ensure_newline "$api_env"
set_url_host "$api_env" APP_BASE_URL "http://$ip_addr:8000"
set_url_host "$api_env" S3_PUBLIC_ENDPOINT "http://$ip_addr:9100"
# CORS: only literal private IPv4 hosts are rewritten; localhost entries stay.
sed -i -E "/^CORS_ORIGINS=/ s#//(10|172|192)\.[0-9]+\.[0-9]+\.[0-9]+#//$ip_addr#g" "$api_env"

touch "$mobile_env"
ensure_newline "$mobile_env"
set_value "$mobile_env" REACT_NATIVE_PACKAGER_HOSTNAME "$ip_addr"
if grep -qE '^EXPO_PUBLIC_API_URL=[a-z]+://' "$mobile_env"; then
  set_url_host "$mobile_env" EXPO_PUBLIC_API_URL ""
fi

echo "LAN address: $ip_addr"
echo "  apps/api/.env:          $(grep -E '^APP_BASE_URL=' "$api_env" | cut -d' ' -f1), $(grep -E '^S3_PUBLIC_ENDPOINT=' "$api_env" | cut -d' ' -f1)"
echo "  apps/mobile/.env.local: REACT_NATIVE_PACKAGER_HOSTNAME=$ip_addr"
if ! grep -qE '^MINIO_BIND=0\.0\.0\.0' "$root/infra/compose/.env" 2>/dev/null; then
  echo "  note: MinIO listens on localhost only — set MINIO_BIND=0.0.0.0 in infra/compose/.env and task dev:up"
fi
echo "Restart the API and the worker; Expo — with --clear. Phone: http://$ip_addr:8000/healthz"
