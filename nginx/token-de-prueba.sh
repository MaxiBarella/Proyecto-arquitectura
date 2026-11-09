#!/bin/sh
# Genera un token de prueba, firmado igual que lo va a firmar Usuarios (15.5, 15.13).
# Sirve para probar el gateway y para k6 mientras Usuarios no emite tokens (15.20).
#
# Uso, desde la raíz del repo:
#   nginx/token-de-prueba.sh airline AR       aerolínea AR, dura 1 hora
#   nginx/token-de-prueba.sh operator         operador, dura 1 hora
#   nginx/token-de-prueba.sh airline LA 60    aerolínea LA, dura 60 segundos
#
# La clave sale de JWT_SECRET o, si no está, del archivo .env.
set -eu

uso() {
    echo "uso: $0 airline <código> [segundos] | $0 operator [segundos]" >&2
    exit 2
}

[ $# -ge 1 ] || uso
rol=$1
case "$rol" in
    airline)
        [ $# -ge 2 ] || uso
        aerolinea=$2
        segundos=${3:-3600}
        datos_aerolinea=",\"airline\":\"$aerolinea\""
        sub="prueba-$aerolinea"
        ;;
    operator)
        segundos=${2:-3600}
        datos_aerolinea=""
        sub="prueba-operador"
        ;;
    *) uso ;;
esac

if [ -z "${JWT_SECRET:-}" ] && [ -f .env ]; then
    JWT_SECRET=$(grep '^JWT_SECRET=' .env | tail -n 1 | cut -d= -f2-)
fi
if [ -z "${JWT_SECRET:-}" ]; then
    echo "$0: falta JWT_SECRET (en el entorno o en .env)" >&2
    exit 1
fi

base64url() {
    openssl base64 -A | tr '+/' '-_' | tr -d '='
}

ahora=$(date +%s)
cabecera=$(printf '%s' '{"alg":"HS256","typ":"JWT"}' | base64url)
datos=$(printf '{"sub":"%s","role":"%s"%s,"iat":%s,"exp":%s}' \
    "$sub" "$rol" "$datos_aerolinea" "$ahora" "$((ahora + segundos))" | base64url)
firma=$(printf '%s' "$cabecera.$datos" | openssl dgst -sha256 -hmac "$JWT_SECRET" -binary | base64url)

echo "$cabecera.$datos.$firma"
