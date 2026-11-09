#!/bin/sh
# Frena el arranque de NGINX si JWT_SECRET falta o tiene menos de 32 bytes (15.21).
# La imagen oficial corre los scripts de /docker-entrypoint.d/ antes de arrancar NGINX,
# y si uno falla, el contenedor termina con error.
set -eu

largo=$(printf '%s' "${JWT_SECRET:-}" | wc -c | tr -d ' ')
if [ "$largo" -lt 32 ]; then
    echo "$0: JWT_SECRET tiene $largo bytes y hacen falta al menos 32 (15.21)." >&2
    echo "$0: generá una con: openssl rand -base64 32" >&2
    exit 1
fi
