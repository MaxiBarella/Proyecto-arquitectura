package queries

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeBoard struct {
	got BoardSearch
	err error
}

func (f *fakeBoard) Search(_ context.Context, s BoardSearch) ([]IndexedFlight, int64, error) {
	f.got = s
	return nil, 0, f.err
}

// 01:00 del 11/11 en UTC son las 22:00 del 10/11 en Argentina.
var consulta = time.Date(2026, 11, 11, 1, 0, 0, 0, time.UTC)

// Sin fecha, de hoy en adelante, con "hoy" en hora de Argentina (RN-33, CA-38).
func TestBoardWithoutDateIsFromToday(t *testing.T) {
	idx := &fakeBoard{}
	if _, _, err := New(nil).WithBoard(idx).Board(context.Background(), BoardFilter{}, consulta); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 11, 10, 3, 0, 0, 0, time.UTC) // 00:00 del 10/11 en Argentina
	if !idx.got.From.Equal(want) || !idx.got.To.IsZero() {
		t.Errorf("rango: %v a %v, want desde %v sin fin", idx.got.From, idx.got.To, want)
	}
	if idx.got.Page != 1 || idx.got.Sort != SortByTime {
		t.Errorf("valores por defecto: %+v", idx.got)
	}
}

// Con fecha, ese día entero en hora de Argentina (14.9, CA-37).
func TestBoardWithDateIsThatDay(t *testing.T) {
	idx := &fakeBoard{}
	day, err := ParseBoardDate("2026-10-23")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := New(nil).WithBoard(idx).Board(context.Background(), BoardFilter{Date: day, Page: 2}, consulta); err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 10, 23, 3, 0, 0, 0, time.UTC)
	if !idx.got.From.Equal(from) || !idx.got.To.Equal(from.Add(24*time.Hour)) || idx.got.Page != 2 {
		t.Errorf("búsqueda: %+v", idx.got)
	}
}

func TestBoardUnavailable(t *testing.T) {
	_, _, err := New(nil).WithBoard(&fakeBoard{err: errors.New("solr caído")}).Board(context.Background(), BoardFilter{}, consulta)
	if !errors.Is(err, ErrBoardUnavailable) {
		t.Errorf("error: %v", err)
	}
	if _, _, err := New(nil).Board(context.Background(), BoardFilter{}, consulta); !errors.Is(err, ErrBoardUnavailable) {
		t.Errorf("sin índice: %v", err)
	}
}
