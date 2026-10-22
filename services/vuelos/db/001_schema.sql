-- Tablas de Vuelos (ADR-003, 5.5). MySQL corre este archivo una sola vez, cuando
-- crea la base. Si cambia, hay que borrar el volumen: docker compose down -v.

CREATE TABLE flights (
  id            CHAR(36)     NOT NULL,
  flight_number VARCHAR(8)   NOT NULL,
  airline       VARCHAR(8)   NOT NULL,
  type          VARCHAR(16)  NOT NULL,
  scope         VARCHAR(16)  NOT NULL,
  origin        VARCHAR(100) NOT NULL,
  destination   VARCHAR(100) NOT NULL,
  scheduled_at  DATETIME     NOT NULL,
  status        VARCHAR(16)  NOT NULL,
  -- El slot: pista y comienzo del bloque (4.11). Los dos están o faltan juntos.
  runway_id     VARCHAR(16)  NULL,
  block_start   DATETIME     NULL,
  -- Aeropuerto alternativo, solo para un arribo desviado (RN-25).
  diverted_to   CHAR(4)      NULL,
  -- Sube con cada cambio. Va en los eventos como versión del dato (4.8).
  version       BIGINT       NOT NULL,
  created_at    DATETIME(6)  NOT NULL,
  updated_at    DATETIME(6)  NOT NULL,
  PRIMARY KEY (id),
  -- Para listar los vuelos de una aerolínea por horario (F-07).
  INDEX idx_flights_airline (airline, scheduled_at),
  CONSTRAINT chk_flights_type CHECK (type IN ('arrival', 'departure')),
  CONSTRAINT chk_flights_scope CHECK (scope IN ('domestic', 'international')),
  CONSTRAINT chk_flights_status CHECK (status IN ('pending_slot', 'scheduled', 'delayed', 'completed', 'diverted', 'cancelled')),
  CONSTRAINT chk_flights_slot CHECK ((runway_id IS NULL) = (block_start IS NULL))
);

-- Eventos pendientes de publicar, guardados en la misma transacción que el cambio
-- del vuelo (4.4, 4.7). El relay los publica en orden y los borra.
CREATE TABLE outbox (
  id             CHAR(36)    NOT NULL,
  type           VARCHAR(64) NOT NULL,
  occurred_at    DATETIME(6) NOT NULL,
  correlation_id VARCHAR(64) NOT NULL,
  -- La traza en curso al guardar el evento: el relay publica dentro de ella (11.6).
  traceparent    VARCHAR(64) NULL,
  data_version   BIGINT      NOT NULL,
  data           JSON        NOT NULL,
  PRIMARY KEY (id),
  INDEX idx_outbox_occurred (occurred_at)
);

-- Mensajes ya aplicados, para no aplicar dos veces el mismo (4.9).
CREATE TABLE processed_messages (
  message_id   CHAR(36)    NOT NULL,
  processed_at DATETIME(6) NOT NULL,
  PRIMARY KEY (message_id)
);
