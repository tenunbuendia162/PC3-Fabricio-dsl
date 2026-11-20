package cli

import (
	"bytes"
	"strings"
	"testing"

	"prj_withdrawal/internal/domain"
	"prj_withdrawal/internal/repository"
	"prj_withdrawal/internal/service"
)

func ejecutar(t *testing.T, entrada string) string {
	t.Helper()
	svc, _ := service.New(repository.NewMemoryStore(domain.Seed()))
	var out bytes.Buffer
	New(svc, strings.NewReader(entrada), &out, "../../features").Run()
	return out.String()
}

func TestSesionRetiroConReintento(t *testing.T) {
	// login, retirar 500 (rechazado), 55 (no múltiplo), 200 (ok), salir
	out := ejecutar(t, "1\n1001\n1234\n1\n500\n55\n200\n0\n0\n")
	for _, want := range []string{"Bienvenido(a), Alice", "[RECHAZADA] Saldo insuficiente", "[ERROR] El monto 55.00", "Retiro exitoso", "Nuevo saldo: S/ 34.56"} {
		if !strings.Contains(out, want) {
			t.Errorf("falta %q en la salida", want)
		}
	}
}

func TestCancelarNoHaceMovimientos(t *testing.T) {
	out := ejecutar(t, "1\n1001\n1234\n1\nc\n4\n0\n0\n")
	if !strings.Contains(out, "Operación cancelada") || !strings.Contains(out, "Sin movimientos") {
		t.Fatal(out)
	}
}

func TestLoginFallidoTresVeces(t *testing.T) {
	out := ejecutar(t, "1\n1001\n0\n1001\n1\n1001\n2\n0\n")
	if !strings.Contains(out, "intento 3 de 3") || !strings.Contains(out, "Demasiados intentos") {
		t.Fatal(out)
	}
}

func TestEOFNoCuelga(t *testing.T) { ejecutar(t, "1\n1001\n") }

func TestDinamico(t *testing.T) {
	var b bytes.Buffer
	m := func(s string) domain.Centavos { v, _ := domain.ParseMonto(s); return v }
	casos := []struct {
		saldo, retiro string
		test          string
		ok            bool
	}{
		{"500", "150", "", true}, {"50", "100", "fallo", true}, {"300", "-20", "error", true},
		{"50", "100", "exito", false}, // forzar éxito con datos que deben fallar -> FALLO
	}
	for _, c := range casos {
		if got := Dinamico(&b, "X", m(c.saldo), m(c.retiro), nil, c.test); got != c.ok {
			t.Errorf("%+v -> %v", c, got)
		}
	}
	ab := m("25") // no es múltiplo de billetes: el modo automático lo clasifica como error
	if !Dinamico(&b, "X", m("100"), 0, &ab, "") || Dinamico(&b, "X", m("100"), 0, &ab, "abono") {
		t.Error("escenarios de abono")
	}
}
