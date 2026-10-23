package aplicacion

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/domain"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/ports"
)

type outboxEnMemoria struct {
	pendientes []ports.EventoGuardado
	marcados   []int64
}

func (o *outboxEnMemoria) Pendientes(context.Context, int) ([]ports.EventoGuardado, error) {
	var res []ports.EventoGuardado
	for _, e := range o.pendientes {
		if !o.marcado(e.Secuencia) {
			res = append(res, e)
		}
	}
	return res, nil
}

func (o *outboxEnMemoria) MarcarPublicado(_ context.Context, secuencia int64) error {
	o.marcados = append(o.marcados, secuencia)
	return nil
}

func (o *outboxEnMemoria) marcado(s int64) bool {
	for _, m := range o.marcados {
		if m == s {
			return true
		}
	}
	return false
}

type publicadorDePrueba struct {
	publicados []string
	fallarEn   string
}

func (p *publicadorDePrueba) Publicar(_ context.Context, e ports.EventoGuardado) error {
	if e.ID == p.fallarEn {
		return errors.New("RabbitMQ no responde")
	}
	p.publicados = append(p.publicados, e.ID)
	return nil
}

func evento(id string, secuencia int64) ports.EventoGuardado {
	return ports.EventoGuardado{Evento: domain.Evento{ID: id, Tipo: domain.EventoSlotAsignado}, Secuencia: secuencia}
}

var logMudo = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestRelayPublicaEnOrdenYMarca(t *testing.T) {
	o := &outboxEnMemoria{pendientes: []ports.EventoGuardado{evento("a", 1), evento("b", 2)}}
	p := &publicadorDePrueba{}

	NuevoRelay(o, p, logMudo).UnaVuelta(context.Background())

	if len(p.publicados) != 2 || p.publicados[0] != "a" || p.publicados[1] != "b" {
		t.Fatalf("se esperaba publicar a y b en orden, vino %v", p.publicados)
	}
	if len(o.marcados) != 2 {
		t.Fatalf("se esperaban 2 marcados, hay %d", len(o.marcados))
	}
}

// Si RabbitMQ falla, el evento queda pendiente y no se publica nada posterior, para no desordenar.
func TestRelayCortaSiFallaYReintentaDespues(t *testing.T) {
	o := &outboxEnMemoria{pendientes: []ports.EventoGuardado{evento("a", 1), evento("b", 2), evento("c", 3)}}
	p := &publicadorDePrueba{fallarEn: "b"}
	r := NuevoRelay(o, p, logMudo)

	r.UnaVuelta(context.Background())
	if len(p.publicados) != 1 || len(o.marcados) != 1 {
		t.Fatalf("tenía que publicar solo a: publicados %v, marcados %v", p.publicados, o.marcados)
	}

	p.fallarEn = ""
	r.UnaVuelta(context.Background())
	if len(p.publicados) != 3 || p.publicados[1] != "b" || p.publicados[2] != "c" {
		t.Fatalf("en la vuelta siguiente tenía que publicar b y c, vino %v", p.publicados)
	}
}
