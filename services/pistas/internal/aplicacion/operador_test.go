package aplicacion

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/domain"
)

func eventosDe(repo *repoEnMemoria, tipo string) []domain.Evento {
	var res []domain.Evento
	for _, e := range repo.eventos {
		if e.Tipo == tipo {
			res = append(res, e)
		}
	}
	return res
}

// F-10, RN-16, CA-11: el operador cierra una pista a mano; sus vuelos de ahí en adelante pierden
// el slot como con el clima, y la otra pista no cambia.
func TestOperadorCierraUnaPista(t *testing.T) {
	repo, p := pistasConVuelos(t)

	pista, err := p.CerrarPista(context.Background(), "05-23")
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if pista.Abierta || pista.CerradaPor != domain.CerradaPorOperador {
		t.Fatalf("la pista tenía que quedar cerrada por el operador: %+v", pista)
	}
	if !repo.pistas[1].Abierta {
		t.Errorf("la 18-36 no tenía que cambiar: %+v", repo.pistas[1])
	}
	lib := liberados(repo.eventos)
	if len(lib) != 2 || lib["DESPEGUE"] != nil || lib["ARRIBO"] == nil || *lib["ARRIBO"] != "SAOU" {
		t.Errorf("se esperaba liberar el despegue y desviar el arribo a SAOU: %v", lib)
	}
	cerradas := eventosDe(repo, domain.EventoPistaCerrada)
	if len(cerradas) != 1 || cerradas[0].Datos.(domain.DatosPista).Motivo != domain.CerradaPorOperador {
		t.Errorf("se esperaba una pista.cerrada por el operador: %+v", cerradas)
	}
}

// Cerrar una pista que ya cerró el operador no cambia nada ni publica eventos (23.3).
func TestCerrarDosVecesNoCambiaNada(t *testing.T) {
	repo, p := pistasConVuelos(t)
	_, _ = p.CerrarPista(context.Background(), "05-23")
	antes := len(repo.eventos)

	if _, err := p.CerrarPista(context.Background(), "05-23"); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if len(repo.eventos) != antes {
		t.Errorf("la repetición no tenía que publicar eventos: %v", tiposDeEvento(repo.eventos[antes:]))
	}
}

// 23.5 y RN-16: cerrar a mano una pista que cerró el clima la pasa a cerrada por el operador, y
// el clima ya no la reabre.
func TestCerrarUnaCerradaPorElClimaLaPasaAlOperador(t *testing.T) {
	repo, p := pistasConVuelos(t)
	_ = p.ProcesarEvento(context.Background(), aptitud("m1", false))

	pista, err := p.CerrarPista(context.Background(), "05-23")
	if err != nil || pista.CerradaPor != domain.CerradaPorOperador {
		t.Fatalf("se esperaba cerrada por el operador: %+v %v", pista, err)
	}
	_ = p.ProcesarEvento(context.Background(), aptitud("m2", true))
	if repo.pistas[0].Abierta || !repo.pistas[1].Abierta {
		t.Errorf("el clima tenía que reabrir solo la 18-36: %+v", repo.pistas)
	}
}

// F-11: el operador reabre una pista que cerró él.
func TestOperadorReabreUnaPista(t *testing.T) {
	repo, p := pistasConVuelos(t)
	_, _ = p.CerrarPista(context.Background(), "05-23")

	pista, err := p.ReabrirPista(context.Background(), "05-23")
	if err != nil || !pista.Abierta {
		t.Fatalf("la pista tenía que quedar abierta: %+v %v", pista, err)
	}
	reabiertas := eventosDe(repo, domain.EventoPistaReabierta)
	if len(reabiertas) != 1 || reabiertas[0].Datos.(domain.DatosPista).Motivo != domain.CerradaPorOperador {
		t.Errorf("se esperaba una pista.reabierta con motivo operator: %+v", reabiertas)
	}
	// Reabrir una abierta no cambia nada.
	antes := len(repo.eventos)
	if _, err := p.ReabrirPista(context.Background(), "05-23"); err != nil || len(repo.eventos) != antes {
		t.Errorf("reabrir una abierta no tenía que publicar eventos: %v", err)
	}
}

// RN-42 y CA-40: con datos de Clima, una pista que cerró el clima no se reabre a mano.
func TestNoSeReabreUnaCerradaPorElClimaConDatos(t *testing.T) {
	repo, p := pistasConVuelos(t)
	_ = p.ProcesarEvento(context.Background(), aptitud("m1", false))

	_, err := p.ReabrirPista(context.Background(), "05-23")
	if !errors.Is(err, domain.ErrCerradaPorClima) {
		t.Fatalf("se esperaba ErrCerradaPorClima, vino %v", err)
	}
	if repo.pistas[0].Abierta {
		t.Errorf("la pista tenía que seguir cerrada")
	}
}

// 23.6 y CA-42: con datos de Clima que no permiten operar, tampoco se reabre una pista cerrada
// por el operador. Si no, cerrar a mano una que cerró el clima y reabrirla esquivaría la RN-42.
func TestConMalClimaNoSeReabreUnaCerradaPorElOperador(t *testing.T) {
	repo, p := pistasConVuelos(t)
	_ = p.ProcesarEvento(context.Background(), aptitud("m1", false))
	if _, err := p.CerrarPista(context.Background(), "05-23"); err != nil {
		t.Fatal(err)
	}
	antes := len(repo.eventos)

	_, err := p.ReabrirPista(context.Background(), "05-23")
	if !errors.Is(err, domain.ErrCerradaPorClima) {
		t.Fatalf("se esperaba ErrCerradaPorClima, vino %v", err)
	}
	if repo.pistas[0].Abierta || len(repo.eventos) != antes {
		t.Errorf("la pista tenía que seguir cerrada y sin eventos nuevos: %+v", repo.pistas[0])
	}

	// Cuando el clima vuelve a permitir operar, el operador sí la reabre (RN-16).
	_ = p.ProcesarEvento(context.Background(), aptitud("m2", true))
	if pista, err := p.ReabrirPista(context.Background(), "05-23"); err != nil || !pista.Abierta {
		t.Fatalf("con buen clima se tenía que poder reabrir: %+v %v", pista, err)
	}
}

// RN-24, RN-42 y CA-20: sin datos de Clima, el operador sí la reabre a mano.
func TestSinDatosSeReabreUnaCerradaPorElClima(t *testing.T) {
	repo, p := pistasConVuelos(t)
	_ = p.ProcesarEvento(context.Background(), aptitud("m1", false))
	_ = p.ProcesarEvento(context.Background(), sinDatos("m2"))

	pista, err := p.ReabrirPista(context.Background(), "05-23")
	if err != nil || !pista.Abierta {
		t.Fatalf("sin datos se tenía que poder reabrir: %+v %v", pista, err)
	}
	reabiertas := eventosDe(repo, domain.EventoPistaReabierta)
	if len(reabiertas) != 1 || reabiertas[0].Datos.(domain.DatosPista).Motivo != domain.CerradaPorClima {
		t.Errorf("se esperaba una pista.reabierta con motivo weather: %+v", reabiertas)
	}
}

func TestPistaInexistenteDelOperador(t *testing.T) {
	_, p := pistasConVuelos(t)
	if _, err := p.CerrarPista(context.Background(), "99-99"); !errors.Is(err, domain.ErrPistaInexistente) {
		t.Errorf("cerrar: se esperaba ErrPistaInexistente, vino %v", err)
	}
	if _, err := p.ReabrirPista(context.Background(), "99-99"); !errors.Is(err, domain.ErrPistaInexistente) {
		t.Errorf("reabrir: se esperaba ErrPistaInexistente, vino %v", err)
	}
}

// F-09 y 23.5: una pista nueva nace abierta, salvo que el clima no deje operar.
func TestAltaDePista(t *testing.T) {
	repo, p := pistasConVuelos(t)

	nueva, err := p.AltaDePista(context.Background(), "09-27")
	if err != nil || !nueva.Abierta {
		t.Fatalf("tenía que nacer abierta: %+v %v", nueva, err)
	}
	if _, err := p.AltaDePista(context.Background(), "09-27"); !errors.Is(err, domain.ErrPistaYaExiste) {
		t.Errorf("repetida: se esperaba ErrPistaYaExiste, vino %v", err)
	}
	var invalidos domain.ErrDatosInvalidos
	if _, err := p.AltaDePista(context.Background(), "05/23"); !errors.As(err, &invalidos) {
		t.Errorf("con barra: se esperaba ErrDatosInvalidos, vino %v", err)
	}

	_ = p.ProcesarEvento(context.Background(), aptitud("m1", false))
	conTormenta, err := p.AltaDePista(context.Background(), "12-30")
	if err != nil || conTormenta.Abierta || conTormenta.CerradaPor != domain.CerradaPorClima {
		t.Fatalf("con el clima cerrado tenía que nacer cerrada por el clima: %+v %v", conTormenta, err)
	}
	if len(repo.pistas) != 4 {
		t.Errorf("se esperaban 4 pistas, hay %d", len(repo.pistas))
	}
	// Con la pista nueva, la asignación la ofrece como libre (RN-09).
	_ = p.ProcesarEvento(context.Background(), aptitud("m2", true))
	if _, err := p.Asignar(context.Background(), pedido("NUEVO", domain.Despegue, "09-27", catorce.Add(time.Hour))); err != nil {
		t.Errorf("se tenía que poder pedir un slot en la pista nueva: %v", err)
	}
}

// F-20, RN-26 y CA-22: un alternativo no disponible se saltea al desviar.
func TestAlternativoNoDisponibleSeSaltea(t *testing.T) {
	repo, p := pistasConVuelos(t)

	alt, err := p.CambiarDisponibilidad(context.Background(), "SAOU", false)
	if err != nil || alt.Disponible {
		t.Fatalf("SAOU tenía que quedar no disponible: %+v %v", alt, err)
	}
	if _, err := p.CerrarPista(context.Background(), "05-23"); err != nil {
		t.Fatal(err)
	}
	if d := liberados(repo.eventos)["ARRIBO"]; d == nil || *d != "SAAR" {
		t.Errorf("el arribo tenía que ir a SAAR: %v", d)
	}
	if _, err := p.CambiarDisponibilidad(context.Background(), "SACO", false); !errors.Is(err, domain.ErrAlternativoInexistente) {
		t.Errorf("SACO no es alternativo: se esperaba ErrAlternativoInexistente, vino %v", err)
	}
}

// F-21: la lista de pistas trae el estado del clima, con el aviso de "sin datos".
func TestListarPistasTraeElClima(t *testing.T) {
	_, p := pistasConVuelos(t)
	_ = p.ProcesarEvento(context.Background(), sinDatos("m1"))

	pistas, clima, err := p.ListarPistas(context.Background())
	if err != nil || len(pistas) != 2 || clima.ConDatos {
		t.Fatalf("se esperaban 2 pistas y Clima sin datos: %+v %+v %v", pistas, clima, err)
	}
}
