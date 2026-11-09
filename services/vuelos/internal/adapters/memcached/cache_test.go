package memcached

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/commands"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/queries"
)

var from = time.Date(2026, 11, 10, 3, 0, 0, 0, time.UTC)

func search(q string, page int) queries.BoardSearch {
	return queries.BoardSearch{BoardFilter: queries.BoardFilter{Query: q, Sort: queries.SortByTime, Page: page}, From: from}
}

// La misma búsqueda da la misma clave, y la clave sirve para Memcached aunque el
// texto tenga espacios o sea largo (ADR-007).
func TestKey(t *testing.T) {
	if Key(search("Buenos Ai", 1)) != Key(search("buenos ai", 1)) {
		t.Error("mayúsculas distintas tienen que dar la misma clave (14.8)")
	}
	if Key(search("bue", 1)) == Key(search("bue", 2)) {
		t.Error("otra página tiene que dar otra clave")
	}
	other := search("bue", 1)
	other.To = from.Add(24 * time.Hour)
	if Key(search("bue", 1)) == Key(other) {
		t.Error("otro rango de fechas tiene que dar otra clave")
	}
	k := Key(search(strings.Repeat("buenos aires ", 40), 1))
	if len(k) > 250 || strings.ContainsAny(k, " \n") || !strings.HasPrefix(k, "tablero:") {
		t.Errorf("clave inválida para Memcached: %q", k)
	}
}

type countingIndex struct {
	calls int
	err   error
}

func (c *countingIndex) Search(_ context.Context, s queries.BoardSearch) ([]queries.IndexedFlight, int64, error) {
	c.calls++
	if c.err != nil {
		return nil, 0, c.err
	}
	f := queries.IndexedFlight{FlightUpdatedData: commands.FlightUpdatedData{ID: "v1", FlightNumber: "AR1550"}, BoardAt: from, DataVersion: 3}
	return []queries.IndexedFlight{f}, 57, nil
}

// Si Memcached no responde, el tablero sigue funcionando contra el índice (10.4).
func TestWithoutMemcachedGoesToIndex(t *testing.T) {
	// Un puerto donde no escucha nadie.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()

	idx := &countingIndex{}
	c := NewBoardCache(idx, addr)
	start := time.Now()
	for i := 0; i < 2; i++ {
		flights, total, err := c.Search(context.Background(), search("bue", 1))
		if err != nil || total != 57 || len(flights) != 1 {
			t.Fatalf("got %v %d %v", flights, total, err)
		}
	}
	if idx.calls != 2 {
		t.Errorf("sin caché, cada consulta va al índice: %d", idx.calls)
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("no tiene que quedarse esperando a Memcached: %v", time.Since(start))
	}
}

func startMemcached(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("necesita Docker")
	}
	ctx := context.Background()
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "memcached:1.6-alpine",
			ExposedPorts: []string{"11211/tcp"},
			WaitingFor:   wait.ForListeningPort("11211/tcp"),
		},
		Started: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })
	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := ctr.MappedPort(ctx, "11211")
	if err != nil {
		t.Fatal(err)
	}
	return net.JoinHostPort(host, port.Port())
}

// Con Memcached: la primera consulta va al índice y la guarda; la segunda sale de la
// caché, con los mismos datos (ADR-007).
func TestCacheHit(t *testing.T) {
	idx := &countingIndex{}
	c := NewBoardCache(idx, startMemcached(t))
	ctx := context.Background()
	first, total1, err := c.Search(ctx, search("bue", 1))
	if err != nil {
		t.Fatal(err)
	}
	second, total2, err := c.Search(ctx, search("BUE", 1))
	if err != nil {
		t.Fatal(err)
	}
	if idx.calls != 1 {
		t.Errorf("la segunda consulta tenía que salir de la caché: %d llamadas al índice", idx.calls)
	}
	if total1 != total2 || len(second) != 1 || second[0].ID != first[0].ID || !second[0].BoardAt.Equal(from) || second[0].DataVersion != 3 {
		t.Errorf("de la caché: %+v %d", second, total2)
	}
	if _, _, err := c.Search(ctx, search("bue", 2)); err != nil || idx.calls != 2 {
		t.Errorf("otra página va al índice: %d %v", idx.calls, err)
	}
}

// Un error del índice no se guarda: la próxima consulta vuelve a intentar.
func TestIndexErrorIsNotCached(t *testing.T) {
	idx := &countingIndex{err: errors.New("solr caído")}
	c := NewBoardCache(idx, startMemcached(t))
	for i := 0; i < 2; i++ {
		if _, _, err := c.Search(context.Background(), search("bue", 1)); err == nil {
			t.Fatal("tiene que devolver el error del índice")
		}
	}
	if idx.calls != 2 {
		t.Errorf("llamadas al índice: %d", idx.calls)
	}
}
