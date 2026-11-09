-- Tablas de Pistas (ADR-003, 5.5). MySQL corre este archivo una sola vez, cuando
-- crea la base. Si cambia, hay que borrar el volumen: docker compose down -v.

CREATE TABLE pistas (
  -- El nombre sin barra, como '05-23' (4.11).
  id          VARCHAR(10) NOT NULL,
  abierta     BOOLEAN     NOT NULL DEFAULT TRUE,
  -- Quién la cerró: 'weather' o 'operator' (4.12). NULL si está abierta. Una pista cerrada por
  -- el operador no la reabre el clima (RN-16).
  cerrada_por VARCHAR(10) NULL,
  PRIMARY KEY (id),
  CONSTRAINT chk_pistas_cierre CHECK (abierta = (cerrada_por IS NULL)),
  CONSTRAINT chk_pistas_motivo CHECK (cerrada_por IN ('weather', 'operator'))
);

-- Lo último que Pistas sabe del clima de nuestro aeropuerto, por los eventos de Clima (4.2, 9.14).
-- Tiene una sola fila.
CREATE TABLE clima (
  aeropuerto     CHAR(4)     NOT NULL,
  -- Si se puede operar según el último dato, o NULL si todavía no llegó ninguno.
  operable       BOOLEAN     NULL,
  -- FALSE después de "Clima sin datos": las pistas no se cierran ni se reabren solas (RN-24).
  con_datos      BOOLEAN     NOT NULL DEFAULT TRUE,
  actualizado_en DATETIME(6) NULL,
  PRIMARY KEY (aeropuerto)
);

-- Aeropuertos a los que se desvía un arribo cuando se cierra su pista, por distancia desde
-- Córdoba (RN-25). El operador puede marcar uno como no disponible (RN-26).
CREATE TABLE aeropuertos_alternativos (
  icao       CHAR(4)     NOT NULL,
  nombre     VARCHAR(40) NOT NULL,
  orden      INT         NOT NULL,
  disponible BOOLEAN     NOT NULL DEFAULT TRUE,
  PRIMARY KEY (icao),
  UNIQUE KEY ux_alternativo_orden (orden)
);

-- Un slot ocupado. Un bloque libre no tiene fila.
CREATE TABLE slots (
  id             BIGINT       NOT NULL AUTO_INCREMENT,
  pista_id       VARCHAR(10)  NOT NULL,
  -- Comienzo del bloque, en UTC y en múltiplos de 15 minutos (RN-03).
  inicio_bloque  DATETIME     NOT NULL,
  vuelo_id       VARCHAR(64)  NOT NULL,
  tipo_operacion VARCHAR(10)  NOT NULL,
  creado_en      DATETIME(6)  NOT NULL,
  PRIMARY KEY (id),
  -- Un slot admite un solo vuelo (RN-04, RN-10).
  UNIQUE KEY ux_slot_pista_bloque (pista_id, inicio_bloque),
  -- Un vuelo tiene a lo sumo un slot: es lo que hace idempotente el pedido (4.6).
  UNIQUE KEY ux_slot_vuelo (vuelo_id),
  -- Para buscar los libres más cercanos y los bloques de un rango (RN-09).
  INDEX ix_slot_inicio (inicio_bloque),
  CONSTRAINT fk_slot_pista FOREIGN KEY (pista_id) REFERENCES pistas (id)
);

-- Eventos pendientes de publicar (4.4, 4.7). secuencia da el orden y es el
-- data_version del mensaje (4.10).
CREATE TABLE outbox (
  secuencia      BIGINT      NOT NULL AUTO_INCREMENT,
  id             CHAR(36)    NOT NULL,
  tipo           VARCHAR(50) NOT NULL,
  fecha          DATETIME(6) NOT NULL,
  correlacion_id VARCHAR(64) NOT NULL,
  -- La traza del pedido que lo causó; el relay la pone en el mensaje (11.6).
  -- Vacío si el evento no salió de un pedido con traza.
  traceparent    VARCHAR(55) NOT NULL DEFAULT '',
  datos          JSON        NOT NULL,
  publicado_en   DATETIME(6) NULL,
  PRIMARY KEY (secuencia),
  UNIQUE KEY ux_outbox_id (id),
  -- El relay busca los que no se publicaron.
  INDEX ix_outbox_publicado (publicado_en)
);

-- Mensajes ya aplicados, para no aplicar dos veces el mismo (4.9).
CREATE TABLE mensajes_procesados (
  id           CHAR(36)    NOT NULL,
  procesado_en DATETIME(6) NOT NULL,
  PRIMARY KEY (id)
);

-- Las dos pistas con las que arranca el sistema (RN-02).
INSERT INTO pistas (id, abierta) VALUES ('05-23', TRUE), ('18-36', TRUE);

-- Nuestro aeropuerto (2.8), sin datos de clima hasta el primer evento.
INSERT INTO clima (aeropuerto, operable, con_datos) VALUES ('SACO', NULL, TRUE);

-- La lista de alternativos (18, pendiente 15).
INSERT INTO aeropuertos_alternativos (icao, nombre, orden) VALUES
  ('SAOU', 'San Luis', 1),
  ('SAAR', 'Rosario', 2),
  ('SAME', 'Mendoza', 3),
  ('SABE', 'Aeroparque', 4),
  ('SAEZ', 'Ezeiza', 5);
