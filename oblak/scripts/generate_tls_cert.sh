#!/usr/bin/env bash
set -euo pipefail

mkdir -p certs

openssl req -x509 -newkey rsa:4096 -sha256 -days 365 -nodes \
  -keyout certs/server.key \
  -out certs/server.crt \
  -subj "/CN=localhost" \
  -addext "subjectAltName=DNS:localhost,IP:127.0.0.1,IP:::1"

chmod 600 certs/server.key
chmod 644 certs/server.crt

echo "TLS sertifikat je generisan u certs/server.crt"
echo "Za lokalni CLI koristi jedan od ova dva pristupa:"
echo "  export OBLAK_CA_CERT=./certs/server.crt"
echo "  ili samo za demo: export OBLAK_INSECURE_SKIP_VERIFY=true"
echo "Server zatim pokreni normalno: sudo ./oblak-server"
