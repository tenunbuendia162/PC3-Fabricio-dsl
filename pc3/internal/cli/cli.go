// Package cli implementa el cajero automático interactivo por terminal.
// Recibe io.Reader/io.Writer para poder probarse sin teclado real.
package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"prj_withdrawal/internal/bdd"
	"prj_withdrawal/internal/domain"
	"prj_withdrawal/internal/gherkin"
	"prj_withdrawal/internal/service"
)

type App struct {
	Svc         *service.Service
	In          *bufio.Reader
	Out         io.Writer
	FeaturesDir string
}

func New(svc *service.Service, in io.Reader, out io.Writer, featuresDir string) *App {
	return &App{Svc: svc, In: bufio.NewReader(in), Out: out, FeaturesDir: featuresDir}
}

func (a *App) p(format string, args ...any) { fmt.Fprintf(a.Out, format, args...) }

// ask muestra un prompt y lee una línea. ok=false si se cerró la entrada (EOF).
func (a *App) ask(prompt string) (string, bool) {
	a.p("%s", prompt)
	line, err := a.In.ReadString('\n')
	if err != nil && line == "" {
		a.p("\n")
		return "", false
	}
	return strings.TrimSpace(line), true
}

// Run ejecuta el menú principal.
func (a *App) Run() {
	a.p("\n╔══════════════════════════════════════════╗\n")
	a.p("║   CAJERO AUTOMÁTICO UNI · prj_withdrawal  ║\n")
	a.p("╚══════════════════════════════════════════╝\n")
	for {
		a.p("\n--- MENÚ PRINCIPAL ---\n 1. Iniciar sesión\n 2. Ejecutar pruebas BDD (Gherkin)\n 0. Salir\n")
		op, ok := a.ask("Opción: ")
		if !ok {
			return
		}
		switch op {
		case "1":
			if !a.login() {
				return
			}
		case "2":
			a.runBDD()
		case "0":
			a.p("Gracias por usar el cajero. ¡Hasta pronto!\n")
			return
		default:
			a.p("Opción inválida.\n")
		}
	}
}

func (a *App) runBDD() {
	res, err := bdd.Run(a.FeaturesDir)
	if err != nil {
		a.p("No se pudieron cargar los .feature (%v). Use -features <carpeta>.\n", err)
		return
	}
	gherkin.Report(a.Out, res)
}

// login devuelve false si se cerró la entrada.
func (a *App) login() bool {
	a.p("(Cuentas demo: 1001 Alice/1234 · 1002 Hamza/4321 · 1003 Carlos/1111 · 1004 María/2222 · 1005 Pedro/3333)\n")
	for intento := 1; intento <= 3; intento++ {
		id, ok := a.ask("N.º de cuenta: ")
		if !ok {
			return false
		}
		pin, ok := a.ask("PIN: ")
		if !ok {
			return false
		}
		v, err := a.Svc.Autenticar(id, pin)
		if err != nil {
			a.p("✘ %v (intento %d de 3)\n", err, intento)
			continue
		}
		a.p("\n✔ Bienvenido(a), %s.\n", v.Titular)
		return a.sesion(v.ID)
	}
	a.p("Demasiados intentos fallidos. Volviendo al menú principal.\n")
	return true
}

func (a *App) sesion(id string) bool {
	for {
		a.p("\n--- OPERACIONES ---\n 1. Retirar efectivo\n 2. Abonar efectivo\n 3. Consultar saldo\n 4. Ver historial\n 5. Ver historial en JSON\n 0. Cerrar sesión\n")
		op, ok := a.ask("Opción: ")
		if !ok {
			return false
		}
		switch op {
		case "1":
			if !a.operacion(id, domain.Retiro) {
				return false
			}
		case "2":
			if !a.operacion(id, domain.Abono) {
				return false
			}
		case "3":
			v, _ := a.Svc.Cuenta(id)
			a.p("Saldo disponible: S/ %s   (retirado hoy: S/ %s de S/ %s)\n", v.Saldo, v.RetiradoHoy, v.LimiteDiario)
		case "4":
			a.historial(id)
		case "5":
			b, _ := json.MarshalIndent(a.Svc.Historial(id), "", "  ")
			a.p("%s\n", b)
		case "0":
			a.p("Sesión cerrada.\n")
			return true
		default:
			a.p("Opción inválida.\n")
		}
	}
}

func (a *App) historial(id string) {
	h := a.Svc.Historial(id)
	if len(h) == 0 {
		a.p("Sin movimientos.\n")
		return
	}
	a.p("%-10s %-20s %-7s %11s %-10s %11s  %s\n", "ID", "Fecha (UTC)", "Tipo", "Monto", "Estado", "Saldo", "Detalle")
	for _, t := range h {
		a.p("%-10s %-20s %-7s %11s %-10s %11s  %s\n", t.ID, t.Fecha.Format("2006-01-02 15:04:05"), t.Tipo, t.Monto, t.Estado, t.SaldoDespues, t.Mensaje)
	}
}

// operacion implementa el flujo principal y los alternos del caso de uso:
// pedir monto (paso 3) -> validar (4) -> saldo (5-6) -> aplicar (7-9) -> saldo (10).
// Flujo 4.a (saldo insuficiente) y 4.b (monto inválido): nuevo monto o cancelar.
// Devuelve false si se cerró la entrada.
func (a *App) operacion(id string, tipo domain.Tipo) bool {
	verbo := map[domain.Tipo]string{domain.Retiro: "retirar", domain.Abono: "abonar"}[tipo]
	prompt := fmt.Sprintf("Monto a %s (o 'c' para cancelar): ", verbo)
	for {
		txt, ok := a.ask(prompt)
		if !ok {
			return false
		}
		if strings.EqualFold(txt, "c") {
			a.p("Operación cancelada. No se realizó ningún movimiento.\n")
			return true
		}
		m, err := domain.ParseMonto(txt)
		if err != nil {
			a.p("✘ Monto inválido (%v). Ingrese un número como 150 o 150.50.\n", err)
			continue
		}
		var tx domain.Transaccion
		if tipo == domain.Retiro {
			tx, err = a.Svc.Retirar(id, m, domain.CanalATM)
		} else {
			tx, err = a.Svc.Abonar(id, m, domain.CanalATM)
		}
		if err == nil {
			if tipo == domain.Retiro {
				a.p("✔ Retiro exitoso. Dispensando S/ %s en efectivo.\n", tx.Monto)
			} else {
				a.p("✔ Abono exitoso. Se recibieron S/ %s en efectivo.\n", tx.Monto)
			}
			a.p("  Transacción %s · Nuevo saldo: S/ %s\n", tx.ID, tx.SaldoDespues)
			return true
		}
		a.p("✘ [%s] %v", domain.EstadoDe(err), err)
		if tx.ID != "" {
			a.p("  (registrada como %s)", tx.ID)
		}
		a.p("\n")
		if e, ok := err.(*domain.Err); ok && (e.Codigo == domain.ErrSistema.Codigo ||
			e.Codigo == domain.ErrCuentaInactiva.Codigo || e.Codigo == domain.ErrCajeroInoperativo.Codigo ||
			e.Codigo == domain.ErrCajeroSinEfectivo.Codigo) {
			a.p("La operación no puede continuar. Fin del caso de uso.\n")
			return true
		}
		a.p("Puede ingresar otro monto o cancelar.\n")
	}
}

// Stdout helper para main.
func Stdout() io.Writer { return os.Stdout }
