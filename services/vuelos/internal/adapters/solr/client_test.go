package solr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/commands"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/queries"
)

var block = time.Date(2026, 11, 10, 18, 0, 0, 0, time.UTC)

func flight() queries.IndexedFlight {
	return queries.IndexedFlight{
		FlightUpdatedData: commands.FlightUpdatedData{
			ID: "vuelo-1", FlightNumber: "AR1550", Airline: "AR", Type: "departure", Scope: "domestic",
			Origin: "Córdoba", Destination: "Buenos Aires", ScheduledAt: block.Add(-time.Hour), Status: "scheduled",
			Slot: &commands.FlightSlotData{RunwayID: "05-23", BlockStart: block},
		},
		BoardAt:     block,
		DataVersion: 3,
	}
}

func TestUpsertSendsDocument(t *testing.T) {
	var got []map[string]any
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/solr/vuelos/update" {
			t.Errorf("pedido: %s %s", r.Method, r.URL.Path)
		}
		query = r.URL.RawQuery
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("cuerpo: %s", body)
		}
		_, _ = w.Write([]byte(`{"responseHeader":{"status":0}}`))
	}))
	defer srv.Close()

	if err := NewClient(srv.URL+"/solr/vuelos/", time.Second).Upsert(context.Background(), flight()); err != nil {
		t.Fatal(err)
	}
	// Lo escrito se ve en hasta 5 segundos (14.7).
	if !strings.Contains(query, "commitWithin=5000") {
		t.Errorf("query: %s", query)
	}
	if len(got) != 1 {
		t.Fatalf("documentos: %v", got)
	}
	d := got[0]
	if d["id"] != "vuelo-1" || d["data_version"] != float64(3) || d["board_at"] != "2026-11-10T18:00:00Z" ||
		d["slot_runway_id"] != "05-23" || d["origin"] != "Córdoba" {
		t.Errorf("documento: %v", d)
	}
	if _, ok := d["diverted_to"]; ok {
		t.Errorf("sin alternativo, diverted_to no va: %v", d)
	}
}

func TestUpsertFailsWhenSolrFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "core caído", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	err := NewClient(srv.URL, time.Second).Upsert(context.Background(), flight())
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Errorf("tiene que fallar con el código de Solr: %v", err)
	}
}

func TestUpsertTimesOut(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release) // antes de cerrar el servidor, que espera a los handlers
	if err := NewClient(srv.URL, 50*time.Millisecond).Upsert(context.Background(), flight()); err == nil {
		t.Error("tiene que fallar por timeout")
	}
}
