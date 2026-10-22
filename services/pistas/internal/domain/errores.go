package domain

import (
	"errors"
	"fmt"
)

// Errores de negocio. El adaptador HTTP los traduce a los códigos de docs/contracts/pistas.yaml.
var (
	ErrPistaInexistente = errors.New("la pista no existe")
	ErrPistaCerrada     = errors.New("la pista está cerrada")
	ErrVueloConSlot     = errors.New("el vuelo ya tiene otro slot asignado")
)

// ErrDatosInvalidos indica un pedido mal formado.
type ErrDatosInvalidos struct {
	Motivo string
}

func (e ErrDatosInvalidos) Error() string {
	return "datos inválidos: " + e.Motivo
}

// ErrSlotOcupado indica que el slot pedido tiene otro vuelo y no corresponde la prioridad del arribo.
// Trae los slots libres más cercanos (RN-09).
type ErrSlotOcupado struct {
	Libres []SlotLibre
}

func (e ErrSlotOcupado) Error() string {
	return fmt.Sprintf("el slot está ocupado; hay %d libres cercanos", len(e.Libres))
}
