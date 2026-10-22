package repositories

import (
	"testing"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/models"
)

// Estos tests fijan el formato de los mensajes (4.10, 9.14), que es el contrato con
// Pistas. Si cambian, hay que avisarle a la pareja 1.

func TestFormatoDeAptitudCambio(t *testing.T) {
	operable, forced := true, false
	observed := time.Date(2026, 10, 19, 19, 45, 0, 0, time.UTC)
	body, err := encodeMessage(models.PendingEvent{
		ID:            "eb368040-7499-4756-a501-1ccf91128d83",
		Type:          models.EventAptitudeChanged,
		OccurredAt:    time.Date(2026, 10, 19, 19, 51, 18, 0, time.UTC),
		CorrelationID: "4bf92f3577b34da6a3ce929d0e0e4736",
		DataVersion:   1,
		Data:          models.EventData{Airport: "SACO", Operable: &operable, ObservedAt: &observed, Forced: &forced},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":"eb368040-7499-4756-a501-1ccf91128d83","type":"clima.aptitud.cambio","version":1,` +
		`"occurred_at":"2026-10-19T19:51:18Z","correlation_id":"4bf92f3577b34da6a3ce929d0e0e4736","data_version":1,` +
		`"data":{"airport":"SACO","operable":true,"reasons":[],"observed_at":"2026-10-19T19:45:00Z","forced":false}}`
	if string(body) != want {
		t.Errorf("mensaje:\n%s\nse esperaba:\n%s", body, want)
	}
}

func TestFormatoDeSinDatos(t *testing.T) {
	observed := time.Date(2026, 10, 19, 19, 45, 0, 0, time.UTC)
	body, err := encodeMessage(models.PendingEvent{
		ID:            "0b1c2d3e-4f50-4617-8899-aabbccddeeff",
		Type:          models.EventNoData,
		OccurredAt:    time.Date(2026, 10, 19, 20, 16, 0, 0, time.UTC),
		CorrelationID: "0b1c2d3e-4f50-4617-8899-aabbccddeeff",
		DataVersion:   2,
		Data:          models.EventData{Airport: "SACO", ObservedAt: &observed},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":"0b1c2d3e-4f50-4617-8899-aabbccddeeff","type":"clima.sin_datos","version":1,` +
		`"occurred_at":"2026-10-19T20:16:00Z","correlation_id":"0b1c2d3e-4f50-4617-8899-aabbccddeeff","data_version":2,` +
		`"data":{"airport":"SACO","observed_at":"2026-10-19T19:45:00Z"}}`
	if string(body) != want {
		t.Errorf("mensaje:\n%s\nse esperaba:\n%s", body, want)
	}
}
