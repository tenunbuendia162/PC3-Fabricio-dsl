package domain

import "testing"

func TestParseMontoYString(t *testing.T) {
	casos := []struct {
		in   string
		want Centavos
		ok   bool
	}{
		{"234.56", 23456, true}, {"200", 20000, true}, {"200.5", 20050, true},
		{"-20", -2000, true}, {"S/ 10.00", 1000, true}, {".5", 50, true},
		{"abc", 0, false}, {"1.234", 0, false}, {"", 0, false}, {"1e3", 0, false}, {"-", 0, false},
	}
	for _, c := range casos {
		got, err := ParseMonto(c.in)
		if (err == nil) != c.ok || (c.ok && got != c.want) {
			t.Errorf("ParseMonto(%q) = %v, %v", c.in, got, err)
		}
	}
	if s := Centavos(-5).String(); s != "-0.05" {
		t.Errorf("String = %s", s)
	}
	if s := Centavos(23456).String(); s != "234.56" {
		t.Errorf("String = %s", s)
	}
}

func TestSinErroresDeRedondeo(t *testing.T) {
	a, _ := ParseMonto("0.10")
	b, _ := ParseMonto("0.20")
	if a+b != 30 { // con float64: 0.1+0.2 != 0.3
		t.Fatal("suma inexacta")
	}
}

func TestRetirarNoMutaSiFalla(t *testing.T) {
	c := NuevaCuenta("1", "X", "0", 10000)
	caj := Cajero{Operativo: true, EfectivoDisponible: 100000, DenominacionMinima: 1000}
	antes := c
	if err := Retirar(&c, &caj, 20000, CanalATM, "2026-11-19"); err != ErrSaldoInsuficiente {
		t.Fatalf("err = %v", err)
	}
	if c != antes {
		t.Fatal("la cuenta cambió pese al rechazo")
	}
}

func TestLimiteDiarioSeReiniciaAlCambiarDeDia(t *testing.T) {
	c := NuevaCuenta("1", "X", "0", 500000)
	caj := Cajero{Operativo: true, EfectivoDisponible: 1000000, DenominacionMinima: 1000}
	if err := Retirar(&c, &caj, 100000, CanalATM, "2026-11-19"); err != nil {
		t.Fatal(err)
	}
	if err := Retirar(&c, &caj, 1000, CanalATM, "2026-11-19"); err == nil {
		t.Fatal("debió exceder el límite diario")
	}
	if err := Retirar(&c, &caj, 1000, CanalATM, "2026-11-20"); err != nil {
		t.Fatalf("al día siguiente debe permitirse: %v", err)
	}
}

func TestPin(t *testing.T) {
	c := NuevaCuenta("1001", "Alice", "1234", 0)
	if !c.PinValido("1234") || c.PinValido("0000") {
		t.Fatal("validación de PIN incorrecta")
	}
}
