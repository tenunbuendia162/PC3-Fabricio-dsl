package gherkin

import "testing"

const src = `# language: es
Característica: Demo
  Descripción libre
  Antecedentes:
    Dado un fondo
  Regla: R1
    Escenario: Uno
      Dado A
      Entonces B
    Esquema del escenario: Dos
      Dado valor <x>
      Entonces resultado <y>
      Ejemplos:
        | x | y |
        | 1 | 2 |
        | 3 | 4 |
`

func TestParseExpandeEsquemas(t *testing.T) {
	f, err := Parse("demo.feature", src)
	if err != nil {
		t.Fatal(err)
	}
	if f.Name != "Demo" || len(f.Background) != 1 || len(f.Scenarios) != 3 {
		t.Fatalf("%+v", f)
	}
	if f.Scenarios[1].Steps[0].Text != "valor 1" || f.Scenarios[2].Steps[1].Text != "resultado 4" || f.Scenarios[1].Rule != "R1" {
		t.Fatalf("%+v", f.Scenarios)
	}
}

func TestParseErrores(t *testing.T) {
	if _, err := Parse("x", "Característica: X\nEscenario: Y\n  Dado A\n  texto suelto"); err == nil {
		t.Fatal("debió rechazar línea desconocida")
	}
	if _, err := Parse("x", "Característica: X\n| a |"); err == nil {
		t.Fatal("tabla fuera de Ejemplos")
	}
}

type w struct{ n int }

func TestRunnerPasosNoDefinidosYOmitidos(t *testing.T) {
	f, _ := Parse("x", "Característica: X\nEscenario: Y\n  Dado paso ok\n  Cuando paso raro\n  Entonces otro\n")
	r := &Registry[w]{}
	r.Step(`^paso ok$`, func(x *w, _ []string) error { x.n++; return nil })
	res := r.Run(f, func() *w { return &w{} })
	if res[0].Passed || res[0].Steps[1].Status != "no definido" || res[0].Steps[2].Status != "omitido" {
		t.Fatalf("%+v", res[0])
	}
}
