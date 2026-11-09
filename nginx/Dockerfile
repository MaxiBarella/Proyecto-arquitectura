# Gateway y balanceador del sistema (15.1, 15.3): un solo NGINX con dos archivos.
# También sirve el frontend (15.7). Se construye desde la raíz del repo, porque necesita
# nginx/ y frontend/ (ver nginx/Dockerfile.dockerignore).

# 1. Compila el frontend: deja los archivos estáticos en /frontend/dist.
FROM node:24-alpine AS frontend
WORKDIR /frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# 2. NGINX. La imagen oficial ya trae el módulo njs, que corre el script del token (15.4).
FROM nginx:1.28-alpine

RUN rm /etc/nginx/conf.d/default.conf

# Valores del límite de tráfico (15.8). El Docker Compose los puede cambiar.
# El filtro hace que la plantilla reemplace solo las variables GATEWAY_*.
ENV GATEWAY_RATE_LIMIT=20 \
    GATEWAY_RATE_BURST=40 \
    NGINX_ENVSUBST_FILTER=^GATEWAY_

COPY nginx/nginx.conf /etc/nginx/nginx.conf
COPY nginx/balanceo.conf /etc/nginx/conf.d/balanceo.conf
# La imagen reemplaza las variables GATEWAY_* al arrancar y deja el resultado en conf.d/gateway.conf.
COPY nginx/gateway.conf.template /etc/nginx/templates/gateway.conf.template
COPY nginx/jwt.js /etc/nginx/njs/jwt.js
# Corre antes de arrancar NGINX y lo frena si la clave es muy corta (15.21).
COPY --chmod=755 nginx/verifica-jwt-secret.sh /docker-entrypoint.d/05-verifica-jwt-secret.sh
COPY --from=frontend /frontend/dist /usr/share/nginx/html
