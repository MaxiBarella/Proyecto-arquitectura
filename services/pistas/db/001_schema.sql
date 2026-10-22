-- Tablas de Pistas (ADR-003, 5.5). MySQL corre este archivo una sola vez, cuando
-- crea la base. Si cambia, hay que borrar el volumen: docker compose down -v.

CREATE TABLE pistas (
  -- El nombre sin barra, como '05-23' (4.11).
  id      VARCHAR(10) NOT NULL,
  abierta BOOLEAN     NOT NULL DEFAULT TRUE,
  PRIMARY KEY (id)
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

-- Mensajes ya aplicados, para no aplicar dos veces el mismo (4.9). La usan los
-- consumidores de noviembre: vuelo cancelado y clima.
CREATE TABLE mensajes_procesados (
  id           CHAR(36)    NOT NULL,
  procesado_en DATETIME(6) NOT NULL,
  PRIMARY KEY (id)
);

-- Las dos pistas con las que arranca el sistema (RN-02).
INSERT INTO pistas (id, abierta) VALUES ('05-23', TRUE), ('18-36', TRUE);
