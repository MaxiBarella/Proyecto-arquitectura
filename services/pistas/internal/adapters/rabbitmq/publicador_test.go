package rabbitmq

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/domain"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/ports"
)

// El mensaje tiene los campos de la 4.10 y los datos de la 4.12: es lo que lee Vuelos.
func TestCodificarSigueElContrato(t *testing.T) {
	datos, _ := json.Marshal(domain.DatosSlotCedido{
		VueloID:        "despegue-1",
		PistaID:        "05-23",
		InicioBloque:   time.Date(2026, 10, 23, 14, 30, 0, 0, time.UTC),
		CedidoAVueloID: "arribo-1",
	})
	cuerpo, err := Codificar(ports.EventoGuardado{
		Evento: domain.Evento{
			ID:            "0c8f3d7e-6a51-4c1b-9d2e-5f4a3b2c1d0e",
			Tipo:          domain.EventoSlotCedido,
			Fecha:         time.Date(2026, 10, 23, 11, 2, 17, 0, time.FixedZone("ART", -3*3600)),
			CorrelacionID: "4bf92f3577b34da6a3ce929d0e0e4736",
		},
		Secuencia: 128,
		DatosJSON: datos,
	})
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal(cuerpo, &m); err != nil {
		t.Fatal(err)
	}
	esperado := map[string]any{
		"id":             "0c8f3d7e-6a51-4c1b-9d2e-5f4a3b2c1d0e",
		"type":           "pistas.slot.cedido",
		"version":        float64(1),
		"occurred_at":    "2026-10-23T14:02:17Z",
		"correlation_id": "4bf92f3577b34da6a3ce929d0e0e4736",
		"data_version":   float64(128),
	}
	for k, v := range esperado {
		if m[k] != v {
			t.Errorf("%s: vino %v, se esperaba %v", k, m[k], v)
		}
	}
	data, _ := m["data"].(map[string]any)
	if data["flight_id"] != "despegue-1" || data["taken_by_flight_id"] != "arribo-1" ||
		data["runway_id"] != "05-23" || data["block_start"] != "2026-10-23T14:30:00Z" {
		t.Errorf("data: vino %v", data)
	}
}
