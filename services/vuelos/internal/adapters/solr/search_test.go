package solr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/queries"
)

func TestSearchBuildsQueryAndReadsDocs(t *testing.T) {
	var form url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/solr/vuelos/select" {
			t.Errorf("ruta: %s", r.URL.Path)
		}
		_ = r.ParseForm()
		form = r.PostForm
		_, _ = w.Write([]byte(`{"response":{"numFound":57,"docs":[{"id":"v1","flight_number":"AR1550","airline":"AR",
			"type":"departure","scope":"domestic","origin":"Córdoba","destination":"Buenos Aires",
			"scheduled_at":"2026-11-10T17:00:00Z","board_at":"2026-11-10T18:00:00Z","status":"scheduled",
			"slot_runway_id":"05-23","slot_block_start":"2026-11-10T18:00:00Z",
			"created_at":"2026-11-09T10:00:00Z","updated_at":"2026-11-09T11:00:00Z","data_version":3}]}}`))
	}))
	defer srv.Close()

	from := time.Date(2026, 11, 10, 3, 0, 0, 0, time.UTC)
	flights, total, err := NewClient(srv.URL+"/solr/vuelos", time.Second).Search(context.Background(), queries.BoardSearch{
		BoardFilter: queries.BoardFilter{Airline: "AR", Status: "scheduled", Query: "buenos ai", Sort: queries.SortByAirline, Page: 3},
		From:        from, To: from.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if form.Get("q") != "{!field f=busqueda v=$texto}" || form.Get("texto") != "buenos ai" {
		t.Errorf("búsqueda: q=%q texto=%q", form.Get("q"), form.Get("texto"))
	}
	if form.Get("sort") != "airline asc, board_at asc, flight_number asc" || form.Get("start") != "40" || form.Get("rows") != "20" {
		t.Errorf("orden o página: %v", form)
	}
	fq := map[string]bool{}
	for _, v := range form["fq"] {
		fq[v] = true
	}
	for _, want := range []string{"{!term f=airline}AR", "{!term f=status}scheduled", "board_at:[2026-11-10T03:00:00Z TO 2026-11-11T03:00:00Z}"} {
		if !fq[want] {
			t.Errorf("falta el filtro %q en %v", want, form["fq"])
		}
	}
	if total != 57 || len(flights) != 1 {
		t.Fatalf("total %d, vuelos %d", total, len(flights))
	}
	f := flights[0]
	if f.ID != "v1" || f.Slot == nil || f.Slot.RunwayID != "05-23" || f.DataVersion != 3 || f.DivertedTo != nil {
		t.Errorf("vuelo: %+v", f)
	}
}

func TestSearchWithoutTextOrEnd(t *testing.T) {
	var form url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		form = r.PostForm
		_, _ = w.Write([]byte(`{"response":{"numFound":0,"docs":[]}}`))
	}))
	defer srv.Close()
	from := time.Date(2026, 11, 10, 3, 0, 0, 0, time.UTC)
	if _, _, err := NewClient(srv.URL, time.Second).Search(context.Background(),
		queries.BoardSearch{BoardFilter: queries.BoardFilter{Sort: queries.SortByTime, Page: 1}, From: from}); err != nil {
		t.Fatal(err)
	}
	if form.Get("q") != "*:*" || form.Get("start") != "0" || form["fq"][0] != "board_at:[2026-11-10T03:00:00Z TO *}" {
		t.Errorf("consulta: %v", form)
	}
}

func TestSearchFailsWhenSolrFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "caído", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	_, _, err := NewClient(srv.URL, time.Second).Search(context.Background(),
		queries.BoardSearch{BoardFilter: queries.BoardFilter{Sort: queries.SortByTime, Page: 1}})
	if err == nil {
		t.Error("tiene que fallar")
	}
}
