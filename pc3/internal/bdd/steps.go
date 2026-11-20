// Package bdd define los pasos (step definitions) que enlazan el lenguaje
// Gherkin de /features con el servicio real: la especificación ES la prueba.
package bdd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"prj_withdrawal/internal/domain"
	"prj_withdrawal/internal/gherkin"
	"prj_withdrawal/internal/repository"
	"prj_withdrawal/internal/service"
)

// World es el estado de un escenario.
type World struct {
	banco *domain.Banco
	store *repository.MemoryStore
	svc   *service.Service
	tx    domain.Transaccion
	err   error
	did   bool // ya hubo un "Cuando"
}

func NewWorld() *World {
	return &World{banco: &domain.Banco{Cajero: domain.Cajero{
		ID: "ATM-TEST", EfectivoDisponible: 500000, Operativo: true, DenominacionMinima: 1000}}}
}

func (w *World) servicio() (*service.Service, error) {
	if w.svc == nil {
		w.store = repository.NewMemoryStore(w.banco)
		s, err := service.New(w.store)
		if err != nil {
			return nil, err
		}
		s.Now = func() time.Time { return time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC) }
		w.svc = s
	}
	return w.svc, nil
}

func id(nombre string) string { return strings.ToLower(nombre) }

func monto(s string) (domain.Centavos, error) { return domain.ParseMonto(s) }

func (w *World) preparar() error {
	if w.svc != nil {
		return fmt.Errorf("no se puede cambiar el contexto (Dado) después de ejecutar una operación (Cuando)")
	}
	return nil
}

func (w *World) operar(tipo domain.Tipo, nombre, m string, canal domain.Canal) error {
	v, err := monto(m)
	if err != nil {
		return fmt.Errorf("monto %q ilegible en el escenario: %w", m, err)
	}
	s, err := w.servicio()
	if err != nil {
		return err
	}
	if tipo == domain.Retiro {
		w.tx, w.err = s.Retirar(id(nombre), v, canal)
	} else {
		w.tx, w.err = s.Abonar(id(nombre), v, canal)
	}
	w.did = true
	return nil
}

func (w *World) esperarEstado(est domain.EstadoTx, que string) error {
	if w.tx.Estado != est {
		return fmt.Errorf("se esperaba %s %s pero fue %s (%v)", que, est, w.tx.Estado, w.err)
	}
	return nil
}

// Steps crea el registro con todas las definiciones.
func Steps() *gherkin.Registry[World] {
	r := &gherkin.Registry[World]{}

	// ---- Given -----------------------------------------------------------
	r.Step(`^que el cajero ATM (?:está operativo con|tiene solo) (\S+) de efectivo$`, func(w *World, a []string) error {
		v, err := monto(a[0])
		if err != nil {
			return err
		}
		if err := w.preparar(); err != nil {
			return err
		}
		w.banco.Cajero.EfectivoDisponible, w.banco.Cajero.Operativo = v, true
		return nil
	})
	r.Step(`^que el cajero ATM está fuera de servicio$`, func(w *World, _ []string) error {
		if err := w.preparar(); err != nil {
			return err
		}
		w.banco.Cajero.Operativo = false
		return nil
	})
	r.Step(`^(\p{L}+) tiene (\S+) en su cuenta$`, func(w *World, a []string) error {
		v, err := monto(a[1])
		if err != nil {
			return err
		}
		if err := w.preparar(); err != nil {
			return err
		}
		w.banco.Cuentas = append(w.banco.Cuentas, domain.NuevaCuenta(id(a[0]), a[0], "0000", v))
		return nil
	})
	r.Step(`^la cuenta de (\p{L}+) está inactiva$`, func(w *World, a []string) error {
		if err := w.preparar(); err != nil {
			return err
		}
		c := w.banco.Cuenta(id(a[0]))
		if c == nil {
			return fmt.Errorf("no existe la cuenta de %s", a[0])
		}
		c.Activa = false
		return nil
	})
	r.Step(`^que la persistencia falla$`, func(w *World, _ []string) error {
		s, err := w.servicio()
		if err != nil {
			return err
		}
		_ = s
		w.store.FailSave = true
		return nil
	})

	// ---- When ------------------------------------------------------------
	r.Step(`^(\p{L}+) intenta retirar (\S+)$`, func(w *World, a []string) error {
		return w.operar(domain.Retiro, a[0], a[1], domain.CanalATM)
	})
	r.Step(`^(\p{L}+) intenta retirar (\S+) por la app$`, func(w *World, a []string) error {
		return w.operar(domain.Retiro, a[0], a[1], domain.CanalApp)
	})
	r.Step(`^(\p{L}+) intenta abonar (\S+)$`, func(w *World, a []string) error {
		return w.operar(domain.Abono, a[0], a[1], domain.CanalATM)
	})
	r.Step(`^(\p{L}+) intenta abonar (\S+) por la app$`, func(w *World, a []string) error {
		return w.operar(domain.Abono, a[0], a[1], domain.CanalApp)
	})

	// ---- Then ------------------------------------------------------------
	r.Step(`^el retiro es exitoso$`, func(w *World, _ []string) error { return w.esperarEstado(domain.Exitosa, "retiro") })
	r.Step(`^el abono es exitoso$`, func(w *World, _ []string) error { return w.esperarEstado(domain.Exitosa, "abono") })
	r.Step(`^el retiro es rechazado$`, func(w *World, _ []string) error { return w.esperarEstado(domain.Rechazada, "retiro") })
	r.Step(`^la operación termina con error (E-[A-Z-]+)$`, func(w *World, a []string) error {
		if w.err == nil {
			return fmt.Errorf("se esperaba el error %s pero la operación fue exitosa", a[0])
		}
		codigo := w.tx.Codigo
		if e, ok := w.err.(*domain.Err); ok { // p. ej. E-SISTEMA: no llegó a registrarse la transacción
			codigo = e.Codigo
		}
		if codigo != a[0] {
			return fmt.Errorf("se esperaba el código %s pero fue %q (%s)", a[0], codigo, w.tx.Mensaje)
		}
		return nil
	})
	r.Step(`^el mensaje es "([^"]*)"$`, func(w *World, a []string) error {
		if w.tx.Mensaje != a[0] {
			return fmt.Errorf("mensaje esperado %q, obtenido %q", a[0], w.tx.Mensaje)
		}
		return nil
	})
	r.Step(`^el saldo de (\p{L}+) es (\S+)$`, func(w *World, a []string) error {
		want, err := monto(a[1])
		if err != nil {
			return err
		}
		s, err := w.servicio()
		if err != nil {
			return err
		}
		v, err := s.Cuenta(id(a[0]))
		if err != nil {
			return err
		}
		if v.Saldo != want {
			return fmt.Errorf("saldo esperado %s, obtenido %s", want, v.Saldo)
		}
		return nil
	})
	r.Step(`^el efectivo del cajero es (\S+)$`, func(w *World, a []string) error {
		want, err := monto(a[0])
		if err != nil {
			return err
		}
		s, _ := w.servicio()
		if got := s.Cajero().EfectivoDisponible; got != want {
			return fmt.Errorf("efectivo esperado %s, obtenido %s", want, got)
		}
		return nil
	})
	r.Step(`^la última transacción de (\p{L}+) es (RETIRO|ABONO) (EXITOSA|RECHAZADA|ERROR)$`, func(w *World, a []string) error {
		s, _ := w.servicio()
		h := s.Historial(id(a[0]))
		if len(h) == 0 {
			return fmt.Errorf("el historial de %s está vacío", a[0])
		}
		t := h[len(h)-1]
		if string(t.Tipo) != a[1] || string(t.Estado) != a[2] {
			return fmt.Errorf("última transacción: %s %s (esperada %s %s)", t.Tipo, t.Estado, a[1], a[2])
		}
		return nil
	})
	r.Step(`^el historial de (\p{L}+) tiene (\d+) transacci(?:ón|ones)$`, func(w *World, a []string) error {
		want, _ := strconv.Atoi(a[1])
		s, _ := w.servicio()
		if got := len(s.Historial(id(a[0]))); got != want {
			return fmt.Errorf("el historial tiene %d transacciones, se esperaban %d", got, want)
		}
		return nil
	})
	r.Step(`^la transacción registra saldo antes (\S+) y saldo después (\S+)$`, func(w *World, a []string) error {
		antes, err1 := monto(a[0])
		desp, err2 := monto(a[1])
		if err1 != nil || err2 != nil {
			return fmt.Errorf("montos ilegibles")
		}
		if w.tx.SaldoAntes != antes || w.tx.SaldoDespues != desp {
			return fmt.Errorf("registrado antes=%s después=%s", w.tx.SaldoAntes, w.tx.SaldoDespues)
		}
		return nil
	})
	r.Step(`^el historial de (\p{L}+) es JSON válido con (\d+) registros?$`, func(w *World, a []string) error {
		want, _ := strconv.Atoi(a[1])
		s, _ := w.servicio()
		b, err := json.Marshal(s.Historial(id(a[0])))
		if err != nil {
			return err
		}
		var back []map[string]any
		if err := json.Unmarshal(b, &back); err != nil {
			return fmt.Errorf("JSON inválido: %w", err)
		}
		if len(back) != want {
			return fmt.Errorf("el JSON tiene %d registros, se esperaban %d", len(back), want)
		}
		for _, k := range []string{"id", "fecha", "tipo", "monto", "estado", "saldo_antes", "saldo_despues"} {
			if _, ok := back[0][k]; !ok {
				return fmt.Errorf("al registro JSON le falta el campo %q", k)
			}
		}
		return nil
	})
	return r
}

// Run carga los .feature de dir y los ejecuta contra el servicio real.
func Run(dir string) ([]gherkin.ScenarioResult, error) {
	feats, err := gherkin.LoadDir(dir)
	if err != nil {
		return nil, err
	}
	reg := Steps()
	var all []gherkin.ScenarioResult
	for _, f := range feats {
		all = append(all, reg.Run(f, NewWorld)...)
	}
	return all, nil
}
