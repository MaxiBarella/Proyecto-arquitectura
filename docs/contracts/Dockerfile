# Mock de la API de Clima, generado con Prism a partir del contrato.
# Construir desde docs/contracts:  docker build -t clima-mock .
# Ejecutar:                        docker run --rm -p 4010:4010 clima-mock
FROM stoplight/prism:5

COPY clima-v1.yaml /contracts/clima-v1.yaml

EXPOSE 4010

CMD ["mock", "-h", "0.0.0.0", "-p", "4010", "-m", "false", "/contracts/clima-v1.yaml"]
