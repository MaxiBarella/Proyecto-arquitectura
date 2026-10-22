package aplicacion

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/domain"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/ports"
)

// repoEnMemoria implementa ports.Repositorio sin base de datos. Cada transacción trabaja sobre una
// copia y solo se aplica si termina sin error, como una transacción real.
type repoEnMemoria struct {
	pistas     []domain.Pista
	slots      []domain.Slot
	eventos    []domain.Evento
	proximoID  int64
	conflictos int // cuántas veces Ocupar va a fallar con ErrConflicto antes de funcionar
}

func nuevoRepo() *repoEnMemoria {
	return &repoEnMemoria{pistas: []domain.Pista{
		{ID: 1, Nombre: "Pista 1", Abierta: true},
		{ID: 2, Nombre: "Pista 2", Abierta: true},
	}}
}

func (r *repoEnMemoria) EnTransaccion(ctx context.Context, fn func(context.Context, ports.Transaccion) error) error {
	tx := &txEnMemoria{
		repo:    r,
		slots:   append([]domain.Slot(nil), r.slots...),
		eventos: append([]domain.Evento(nil), r.eventos...),
		id:      r.proximoID,
	}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	r.slots, r.eventos, r.proximoID = tx.slots, tx.eventos, tx.id
	return nil
}

func (r *repoEnMemoria) Pistas(context.Context) ([]domain.Pista, error) { return r.pistas, nil }

func (r *repoEnMemoria) SlotsOcupados(_ context.Context, desde, hasta time.Time) ([]domain.Slot, error) {
	return filtrar(r.slots, desde, hasta), nil
}

type txEnMemoria struct {
	repo    *repoEnMemoria
	slots   []domain.Slot
	eventos []domain.Evento
	id      int64
}

func (t *txEnMemoria) Pistas(context.Context) ([]domain.Pista, error) { return t.repo.pistas, nil }

func (t *txEnMemoria) SlotsOcupados(_ context.Context, desde, hasta time.Time) ([]domain.Slot, error) {
	return filtrar(t.slots, desde, hasta), nil
}

func (t *txEnMemoria) Pista(_ context.Context, id int64) (domain.Pista, error) {
	for _, p := range t.repo.pistas {
		if p.ID == id {
			return p, nil
		}
	}
	return domain.Pista{}, domain.ErrPistaInexistente
}

func (t *txEnMemoria) SlotDelVuelo(_ context.Context, vueloID string) (*domain.Slot, error) {
	for _, s := range t.slots {
		if s.VueloID == vueloID {
			return &s, nil
		}
	}
	return nil, nil
}

func (t *txEnMemoria) OcupanteConBloqueo(_ context.Context, pistaID int64, inicio time.Time) (*domain.Slot, error) {
	for _, s := range t.slots {
		if s.MismoLugar(pistaID, inicio) {
			return &s, nil
		}
	}
	return nil, nil
}

func (t *txEnMemoria) Ocupar(_ context.Context, s *domain.Slot) error {
	if t.repo.conflictos > 0 {
		t.repo.conflictos--
		return ports.ErrConflicto
	}
	for _, o := range t.slots {
		if o.MismoLugar(s.PistaID, s.InicioBloque) || o.VueloID == s.VueloID {
			return ports.ErrConflicto
		}
	}
	t.id++
	s.ID = t.id
	t.slots = append(t.slots, *s)
	return nil
}

func (t *txEnMemoria) Liberar(_ context.Context, slotID int64) error {
	for i, s := range t.slots {
		if s.ID == slotID {
			t.slots = append(t.slots[:i], t.slots[i+1:]...)
			return nil
		}
	}
	return nil
}

func (t *txEnMemoria) GuardarEvento(_ context.Context, e domain.Evento) error {
	t.eventos = append(t.eventos, e)
	return nil
}

func filtrar(slots []domain.Slot, desde, hasta time.Time) []domain.Slot {
	var res []domain.Slot
	for _, s := range slots {
		if !s.InicioBloque.Before(desde) && s.InicioBloque.Before(hasta) {
			res = append(res, s)
		}
	}
	return res
}

var catorce = time.Date(2026, 10, 23, 14, 0, 0, 0, domain.ZonaCordoba)

func pedido(vuelo string, tipo domain.TipoOperacion, pista int64, inicio time.Time) domain.PedidoDeSlot {
	return domain.PedidoDeSlot{VueloID: vuelo, TipoOperacion: tipo, PistaID: pista, InicioBloque: inicio}
}

func tiposDeEvento(eventos []domain.Evento) []string {
	var tipos []string
	for _, e := range eventos {
		tipos = append(tipos, e.Tipo)
	}
	return tipos
}

// CA-02: un slot libre se asigna y se guarda "slot.asignado" en la misma transacción.
func TestAsignarSlotLibre(t *testing.T) {
	repo := nuevoRepo()
	ctx := ConCorrelacion(context.Background(), "traza-1")

	res, err := NuevoPistas(repo).Asignar(ctx, pedido("AR1550", domain.Despegue, 1, catorce))
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if res.YaAsignado || res.VueloDesplazadoID != "" || res.Slot.ID == 0 {
		t.Fatalf("resultado inesperado: %+v", res)
	}
	if len(repo.eventos) != 1 || repo.eventos[0].Tipo != domain.EventoSlotAsignado {
		t.Fatalf("se esperaba un slot.asignado, vino %v", tiposDeEvento(repo.eventos))
	}
	if repo.eventos[0].CorrelacionID != "traza-1" {
		t.Fatalf("el evento tiene que llevar la correlación del pedido, vino %q", repo.eventos[0].CorrelacionID)
	}
}

// CA-06 y 4.6: repetir el mismo pedido no ocupa otro slot ni publica de nuevo.
func TestPedidoRepetidoEsIdempotente(t *testing.T) {
	repo := nuevoRepo()
	p := NuevoPistas(repo)
	primero, _ := p.Asignar(context.Background(), pedido("AR1550", domain.Despegue, 1, catorce))

	segundo, err := p.Asignar(context.Background(), pedido("AR1550", domain.Despegue, 1, catorce))
	if err != nil {
		t.Fatalf("el pedido repetido no debería fallar: %v", err)
	}
	if !segundo.YaAsignado || segundo.Slot.ID != primero.Slot.ID {
		t.Fatalf("se esperaba el mismo slot marcado como ya asignado: %+v", segundo)
	}
	if len(repo.slots) != 1 || len(repo.eventos) != 1 {
		t.Fatalf("se esperaba 1 slot y 1 evento, hay %d y %d", len(repo.slots), len(repo.eventos))
	}
}

// 18.8: un vuelo con slot no puede pedir otro.
func TestVueloConSlotNoPidaOtro(t *testing.T) {
	p := NuevoPistas(nuevoRepo())
	_, _ = p.Asignar(context.Background(), pedido("AR1550", domain.Despegue, 1, catorce))

	_, err := p.Asignar(context.Background(), pedido("AR1550", domain.Despegue, 2, catorce))
	if !errors.Is(err, domain.ErrVueloConSlot) {
		t.Fatalf("se esperaba ErrVueloConSlot, vino %v", err)
	}
}

// CA-03: un despegue no le quita el slot a nadie, y recibe los libres más cercanos.
func TestDespegueRechazadoConLibresCercanos(t *testing.T) {
	repo := nuevoRepo()
	p := NuevoPistas(repo)
	_, _ = p.Asignar(context.Background(), pedido("AR1000", domain.Arribo, 1, catorce))

	_, err := p.Asignar(context.Background(), pedido("LA2000", domain.Despegue, 1, catorce))
	var ocupado domain.ErrSlotOcupado
	if !errors.As(err, &ocupado) {
		t.Fatalf("se esperaba ErrSlotOcupado, vino %v", err)
	}
	if len(ocupado.Libres) != domain.CantidadDeLibresCercanos {
		t.Fatalf("se esperaban %d libres, vinieron %d", domain.CantidadDeLibresCercanos, len(ocupado.Libres))
	}
	if l := ocupado.Libres[0]; l.PistaID != 2 || !l.InicioBloque.Equal(catorce) {
		t.Fatalf("el más cercano es la pista 2 a la misma hora, vino %+v", l)
	}
	if repo.slots[0].VueloID != "AR1000" {
		t.Fatalf("el slot tiene que seguir con su vuelo original")
	}
}

// CA-04: entre dos arribos gana el primero.
func TestArriboNoLeQuitaElSlotAOtroArribo(t *testing.T) {
	p := NuevoPistas(nuevoRepo())
	_, _ = p.Asignar(context.Background(), pedido("AR1000", domain.Arribo, 1, catorce))

	_, err := p.Asignar(context.Background(), pedido("AR2000", domain.Arribo, 1, catorce))
	var ocupado domain.ErrSlotOcupado
	if !errors.As(err, &ocupado) {
		t.Fatalf("se esperaba ErrSlotOcupado, vino %v", err)
	}
}

// CA-07: un arribo se queda con el slot de un despegue, y se guardan los dos eventos juntos.
func TestArriboLeQuitaElSlotAUnDespegue(t *testing.T) {
	repo := nuevoRepo()
	p := NuevoPistas(repo)
	_, _ = p.Asignar(context.Background(), pedido("LA2000", domain.Despegue, 1, catorce))

	res, err := p.Asignar(context.Background(), pedido("AR1000", domain.Arribo, 1, catorce))
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if res.VueloDesplazadoID != "LA2000" {
		t.Fatalf("se esperaba que desplace a LA2000, vino %q", res.VueloDesplazadoID)
	}
	if len(repo.slots) != 1 || repo.slots[0].VueloID != "AR1000" {
		t.Fatalf("el slot tiene que quedar solo para el arribo: %+v", repo.slots)
	}
	tipos := tiposDeEvento(repo.eventos)
	esperado := []string{domain.EventoSlotAsignado, domain.EventoSlotCedido, domain.EventoSlotAsignado}
	if len(tipos) != len(esperado) {
		t.Fatalf("eventos: vino %v, se esperaba %v", tipos, esperado)
	}
	for i := range esperado {
		if tipos[i] != esperado[i] {
			t.Fatalf("eventos: vino %v, se esperaba %v", tipos, esperado)
		}
	}
	cedido := repo.eventos[1].Datos.(domain.DatosSlotCedido)
	if cedido.VueloID != "LA2000" || cedido.CedidoAVueloID != "AR1000" {
		t.Fatalf("datos del slot.cedido incorrectos: %+v", cedido)
	}
}

// El despegue que perdió su slot puede pedir otro (RN-19, CA-14).
func TestDespegueDesplazadoPuedePedirOtro(t *testing.T) {
	p := NuevoPistas(nuevoRepo())
	_, _ = p.Asignar(context.Background(), pedido("LA2000", domain.Despegue, 1, catorce))
	_, _ = p.Asignar(context.Background(), pedido("AR1000", domain.Arribo, 1, catorce))

	if _, err := p.Asignar(context.Background(), pedido("LA2000", domain.Despegue, 2, catorce)); err != nil {
		t.Fatalf("el despegue demorado tiene que poder pedir un slot nuevo: %v", err)
	}
}

// 18.4: no se puede pedir un slot en una pista cerrada.
func TestPistaCerrada(t *testing.T) {
	repo := nuevoRepo()
	repo.pistas[0].Abierta = false

	_, err := NuevoPistas(repo).Asignar(context.Background(), pedido("AR1000", domain.Arribo, 1, catorce))
	if !errors.Is(err, domain.ErrPistaCerrada) {
		t.Fatalf("se esperaba ErrPistaCerrada, vino %v", err)
	}
}

func TestPistaInexistente(t *testing.T) {
	_, err := NuevoPistas(nuevoRepo()).Asignar(context.Background(), pedido("AR1000", domain.Arribo, 9, catorce))
	if !errors.Is(err, domain.ErrPistaInexistente) {
		t.Fatalf("se esperaba ErrPistaInexistente, vino %v", err)
	}
}

func TestPedidoInvalidoNoTocaLaBase(t *testing.T) {
	repo := nuevoRepo()
	_, err := NuevoPistas(repo).Asignar(context.Background(), pedido("AR1000", domain.Arribo, 1, catorce.Add(7*time.Minute)))
	var invalidos domain.ErrDatosInvalidos
	if !errors.As(err, &invalidos) || len(repo.slots) != 0 {
		t.Fatalf("se esperaba ErrDatosInvalidos sin cambios, vino %v", err)
	}
}

// RN-10: si la asignación choca con otra simultánea, se reintenta y decide con lo que guardó la otra.
func TestReintentaAnteUnConflicto(t *testing.T) {
	repo := nuevoRepo()
	repo.conflictos = 1

	res, err := NuevoPistas(repo).Asignar(context.Background(), pedido("AR1000", domain.Arribo, 1, catorce))
	if err != nil || res.Slot.ID == 0 {
		t.Fatalf("después de un conflicto tiene que reintentar y asignar: %+v, %v", res, err)
	}
}

func TestConflictoPersistenteSeDevuelve(t *testing.T) {
	repo := nuevoRepo()
	repo.conflictos = intentosPorConflicto

	_, err := NuevoPistas(repo).Asignar(context.Background(), pedido("AR1000", domain.Arribo, 1, catorce))
	if !errors.Is(err, ports.ErrConflicto) {
		t.Fatalf("agotados los intentos se esperaba ErrConflicto, vino %v", err)
	}
	if len(repo.slots) != 0 || len(repo.eventos) != 0 {
		t.Fatalf("una transacción fallida no puede dejar nada guardado")
	}
}

// Un bloque de una pista cerrada o con vuelo no figura como libre.
func TestListarSlots(t *testing.T) {
	repo := nuevoRepo()
	repo.pistas[1].Abierta = false
	p := NuevoPistas(repo)
	_, _ = p.Asignar(context.Background(), pedido("AR1000", domain.Arribo, 1, catorce))

	bloques, err := p.ListarSlots(context.Background(), catorce, 0)
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if len(bloques) != 2*96 {
		t.Fatalf("se esperaban 96 bloques por pista, vinieron %d", len(bloques))
	}
	for _, b := range bloques {
		ocupado := b.PistaID == 1 && b.InicioBloque.Equal(catorce)
		esperado := b.PistaID == 1 && !ocupado
		if b.Libre != esperado {
			t.Fatalf("pista %d a las %s: libre = %v, se esperaba %v", b.PistaID, b.InicioBloque.Format("15:04"), b.Libre, esperado)
		}
	}

	soloUna, _ := p.ListarSlots(context.Background(), catorce, 1)
	if len(soloUna) != 96 {
		t.Fatalf("filtrando por pista se esperaban 96 bloques, vinieron %d", len(soloUna))
	}
}
