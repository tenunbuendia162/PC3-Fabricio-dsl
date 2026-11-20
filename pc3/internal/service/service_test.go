package service

import (
	"path/filepath"
	"sync"
	"testing"

	"prj_withdrawal/internal/domain"
	"prj_withdrawal/internal/repository"
)

func nuevo(t *testing.T) (*Service, *repository.MemoryStore) {
	t.Helper()
	st := repository.NewMemoryStore(domain.Seed())
	s, err := New(st)
	if err != nil {
		t.Fatal(err)
	}
	return s, st
}

func TestFlujoPrincipalYAlterno(t *testing.T) {
	s, _ := nuevo(t)
	tx, err := s.Retirar("1001", 20000, domain.CanalATM)
	if err != nil || tx.Estado != domain.Exitosa || tx.SaldoDespues != 3456 {
		t.Fatalf("retiro exitoso: %+v %v", tx, err)
	}
	tx, err = s.Retirar("1002", 20000, domain.CanalATM)
	if err == nil || tx.Estado != domain.Rechazada {
		t.Fatalf("debió rechazarse: %+v", tx)
	}
	tx, _ = s.Retirar("1005", -2000, domain.CanalATM)
	if tx.Estado != domain.Error || tx.Codigo != "E-MONTO" {
		t.Fatalf("debió ser error: %+v", tx)
	}
	if n := len(s.Historial("1001")) + len(s.Historial("1002")) + len(s.Historial("1005")); n != 3 {
		t.Fatalf("historial = %d", n)
	}
}

func TestAtomicidadSiFallaPersistencia(t *testing.T) {
	s, st := nuevo(t)
	st.FailSave = true
	_, err := s.Retirar("1001", 10000, domain.CanalATM)
	if err == nil {
		t.Fatal("debió fallar")
	}
	v, _ := s.Cuenta("1001")
	if v.Saldo != 23456 || len(s.Historial("1001")) != 0 || s.Cajero().EfectivoDisponible != 500000 {
		t.Fatal("quedó una operación a medias")
	}
	st.FailSave = false
	if _, err := s.Retirar("1001", 10000, domain.CanalATM); err != nil {
		t.Fatalf("debe recuperarse: %v", err)
	}
}

func TestConcurrenciaNoGeneraSobregiro(t *testing.T) {
	s, _ := nuevo(t)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); s.Retirar("1003", 10000, domain.CanalApp) }() // saldo 500.00, límite diario 1000.00
	}
	wg.Wait()
	v, _ := s.Cuenta("1003")
	if v.Saldo != 0 {
		t.Fatalf("saldo final = %s, se esperaba 0.00 (5 retiros de 100)", v.Saldo)
	}
	if got := len(s.Historial("1003")); got != 50 {
		t.Fatalf("todos los intentos deben quedar registrados, hay %d", got)
	}
}

func TestPersistenciaJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "banco.json")
	s, err := New(repository.NewJSONStore(path))
	if err != nil {
		t.Fatal(err)
	}
	s.Retirar("1001", 10000, domain.CanalATM)
	s.Retirar("1001", 99999000, domain.CanalATM)
	s2, err := New(repository.NewJSONStore(path)) // "reinicio" del sistema
	if err != nil {
		t.Fatal(err)
	}
	v, _ := s2.Cuenta("1001")
	if v.Saldo != 13456 || len(s2.Historial("1001")) != 2 {
		t.Fatalf("estado no persistido: %+v hist=%d", v, len(s2.Historial("1001")))
	}
}

func TestAutenticar(t *testing.T) {
	s, _ := nuevo(t)
	if _, err := s.Autenticar("1001", "1234"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Autenticar("1001", "9999"); err != domain.ErrAutenticacion {
		t.Fatal("PIN incorrecto debe fallar")
	}
	if _, err := s.Autenticar("nadie", "1234"); err != domain.ErrAutenticacion {
		t.Fatal("cuenta inexistente debe fallar igual")
	}
}
