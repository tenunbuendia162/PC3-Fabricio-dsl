package cli

import (
	"fmt"
	"io"
	"math"

	"prj_withdrawal/internal/domain"
	"prj_withdrawal/internal/repository"
	"prj_withdrawal/internal/service"
)

// Dinamico reproduce el modo por flags del codeLAB-02 (-nombre -saldo -retiro -test)
// ampliado con -abono. Usa memoria (sin tocar el disco). Devuelve true si el resultado
// coincide con lo esperado por el escenario.
//
//	test: "" (automático) | exito | fallo | error | abono | abono-error
func Dinamico(out io.Writer, nombre string, saldo, retiro domain.Centavos, abono *domain.Centavos, test string) bool {
	limite := domain.Centavos(math.Max(100000, float64(saldo)))
	c := domain.NuevaCuenta("1", nombre, "0000", saldo)
	c.LimiteDiario = limite
	st := repository.NewMemoryStore(&domain.Banco{
		Cuentas: []domain.Cuenta{c},
		Cajero:  domain.Cajero{ID: "ATM-DIN", EfectivoDisponible: 100000000, Operativo: true, DenominacionMinima: 1000},
	})
	svc, _ := service.New(st)

	run := func(tipo domain.Tipo, m domain.Centavos) (domain.Transaccion, error) {
		if tipo == domain.Retiro {
			return svc.Retirar("1", m, domain.CanalATM)
		}
		return svc.Abonar("1", m, domain.CanalATM)
	}
	final := func() domain.Centavos { v, _ := svc.Cuenta("1"); return v.Saldo }

	if test == "" { // automático: decide el escenario con el dato
		switch {
		case abono != nil && (*abono <= 0 || *abono%1000 != 0):
			test = "abono-error"
		case abono != nil:
			test = "abono"
		case retiro <= 0:
			test = "error"
		case retiro > saldo:
			test = "fallo"
		default:
			test = "exito"
		}
		fmt.Fprintln(out, "=== EJECUTANDO TEST DINÁMICO AUTOMÁTICO ===")
	}
	ab := domain.Centavos(0)
	if abono != nil {
		ab = *abono
	}

	var esperado domain.EstadoTx
	var tipo domain.Tipo
	var monto domain.Centavos
	var titulo string
	switch test {
	case "exito":
		titulo, esperado, tipo, monto = "Escenario 1: Successful withdrawal within balance", domain.Exitosa, domain.Retiro, retiro
	case "fallo":
		titulo, esperado, tipo, monto = "Escenario 2: Declined withdrawal in excess of balance", domain.Rechazada, domain.Retiro, retiro
	case "error":
		titulo, esperado, tipo, monto = "Escenario 3: Validation Error", domain.Error, domain.Retiro, retiro
	case "abono":
		titulo, esperado, tipo, monto = "Escenario 4: Successful cash deposit", domain.Exitosa, domain.Abono, ab
	case "abono-error":
		titulo, esperado, tipo, monto = "Escenario 5: Cash deposit validation error", domain.Error, domain.Abono, ab
	default:
		fmt.Fprintf(out, "Test desconocido %q. Use: exito, fallo, error, abono, abono-error\n", test)
		return false
	}
	verbo := map[domain.Tipo]string{domain.Retiro: "retirar", domain.Abono: "abonar"}[tipo]

	fmt.Fprintf(out, "--- %s (%s) ---\n", titulo, nombre)
	fmt.Fprintf(out, "Given: %s tiene %s en su cuenta\n", nombre, saldo)
	fmt.Fprintf(out, "When: %s intenta %s %s\n", nombre, verbo, monto)
	tx, err := run(tipo, monto)

	esperadoSaldo := saldo
	if esperado == domain.Exitosa {
		if tipo == domain.Retiro {
			esperadoSaldo = saldo - monto
		} else {
			esperadoSaldo = saldo + monto
		}
	}
	ok := tx.Estado == esperado && final() == esperadoSaldo
	fmt.Fprintf(out, "Then: estado=%s", tx.Estado)
	if err != nil {
		fmt.Fprintf(out, " [%s] %v", tx.Codigo, err)
	}
	fmt.Fprintf(out, "\n      saldo final=%s (esperado %s)\n", final(), esperadoSaldo)
	if ok {
		fmt.Fprintf(out, "Resultado: ÉXITO. El escenario se comportó como se esperaba (%s).\n\n", esperado)
	} else {
		fmt.Fprintf(out, "Resultado: FALLO. Se esperaba %s y se obtuvo %s.\n\n", esperado, tx.Estado)
	}
	return ok
}
