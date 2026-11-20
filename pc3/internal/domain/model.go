package domain

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"time"
)

// Tipo de transacción.
type Tipo string

const (
	Retiro Tipo = "RETIRO"
	Abono  Tipo = "ABONO"
)

// Canal por el que se realiza la operación (modelo: Cajero (canal)).
type Canal string

const (
	CanalATM Canal = "ATM" // físico: exige múltiplos de la denominación y usa el efectivo del cajero
	CanalApp Canal = "APP" // digital
	CanalWeb Canal = "WEB" // digital
)

// Cuenta: Context Cuenta (saldo, activa, limiteDiario, retiradoHoy).
type Cuenta struct {
	ID           string   `json:"id"`
	Titular      string   `json:"titular"`
	PinHash      string   `json:"pin_hash"`
	Saldo        Centavos `json:"saldo"`
	Activa       bool     `json:"activa"`
	LimiteDiario Centavos `json:"limite_diario"`
	RetiradoHoy  Centavos `json:"retirado_hoy"`
	FechaRetiro  string   `json:"fecha_retiro"` // YYYY-MM-DD del acumulado retiradoHoy
}

// Cajero: Context Cajero (efectivoDisponible, operativo, denominacionMinima).
type Cajero struct {
	ID                 string   `json:"id"`
	EfectivoDisponible Centavos `json:"efectivo_disponible"`
	Operativo          bool     `json:"operativo"`
	DenominacionMinima Centavos `json:"denominacion_minima"`
}

// Transaccion: Context Transaccion (tipo, monto, estado) + datos de auditoría (US-04).
type Transaccion struct {
	ID           string    `json:"id"`
	Fecha        time.Time `json:"fecha"`
	Tipo         Tipo      `json:"tipo"`
	CuentaID     string    `json:"cuenta_id"`
	Canal        Canal     `json:"canal"`
	Monto        Centavos  `json:"monto"`
	Estado       EstadoTx  `json:"estado"`
	Codigo       string    `json:"codigo,omitempty"`
	Mensaje      string    `json:"mensaje"`
	SaldoAntes   Centavos  `json:"saldo_antes"`
	SaldoDespues Centavos  `json:"saldo_despues"`
}

// Banco agrupa todo el estado persistible. Se guarda de una sola vez para que
// el saldo y el registro de la transacción sean atómicos (RN-03).
type Banco struct {
	Cuentas       []Cuenta      `json:"cuentas"`
	Cajero        Cajero        `json:"cajero"`
	Secuencia     int           `json:"secuencia"`
	Transacciones []Transaccion `json:"transacciones"`
}

// Clone devuelve una copia profunda (base del commit/rollback).
func (b *Banco) Clone() *Banco {
	c := *b
	c.Cuentas = append([]Cuenta(nil), b.Cuentas...)
	c.Transacciones = append([]Transaccion(nil), b.Transacciones...)
	return &c
}

// Cuenta busca una cuenta por ID; nil si no existe.
func (b *Banco) Cuenta(id string) *Cuenta {
	for i := range b.Cuentas {
		if b.Cuentas[i].ID == id {
			return &b.Cuentas[i]
		}
	}
	return nil
}

// HashPin genera el hash almacenado del PIN (nunca se guarda el PIN en claro).
func HashPin(cuentaID, pin string) string {
	h := sha256.Sum256([]byte(cuentaID + ":" + pin))
	return hex.EncodeToString(h[:])
}

// PinValido compara en tiempo constante.
func (c *Cuenta) PinValido(pin string) bool {
	return subtle.ConstantTimeCompare([]byte(c.PinHash), []byte(HashPin(c.ID, pin))) == 1
}

// NuevaCuenta es el constructor (equivalente a NewCuenta del codeLAB-02).
func NuevaCuenta(id, titular, pin string, saldo Centavos) Cuenta {
	return Cuenta{ID: id, Titular: titular, PinHash: HashPin(id, pin), Saldo: saldo,
		Activa: true, LimiteDiario: 100000}
}

// Seed crea datos de demostración (Alice y Hamza de la guía; Carlos, María y Pedro del CLI).
func Seed() *Banco {
	return &Banco{
		Cajero: Cajero{ID: "ATM-001", EfectivoDisponible: 500000, Operativo: true, DenominacionMinima: 1000},
		Cuentas: []Cuenta{
			NuevaCuenta("1001", "Alice", "1234", 23456),
			NuevaCuenta("1002", "Hamza", "4321", 19876),
			NuevaCuenta("1003", "Carlos", "1111", 50000),
			NuevaCuenta("1004", "María", "2222", 5000),
			NuevaCuenta("1005", "Pedro", "3333", 30000),
		},
	}
}

func (c *Cuenta) String() string { return fmt.Sprintf("Cuenta{%s, saldo=%s}", c.Titular, c.Saldo) }

// ---------------------------------------------------------------------------
// Reglas de negocio puras (sin I/O). Validan TODO antes de mutar: así una
// operación fallida nunca deja la cuenta a medias (RN-03).
// ---------------------------------------------------------------------------

func validarMonto(m Centavos, canal Canal, caj *Cajero) error {
	if m <= 0 {
		return ErrMonto.Con("El monto debe ser mayor que cero (recibido %s)", m)
	}
	if canal == CanalATM && caj.DenominacionMinima > 0 && m%caj.DenominacionMinima != 0 {
		return ErrDenominacion.Con("El monto %s no es múltiplo de los billetes disponibles (%s)", m, caj.DenominacionMinima)
	}
	return nil
}

// PuedeRetirar valida el retiro sin modificar nada. hoy = "YYYY-MM-DD".
func PuedeRetirar(c *Cuenta, caj *Cajero, m Centavos, canal Canal, hoy string) error {
	if canal == CanalATM && !caj.Operativo {
		return ErrCajeroInoperativo
	}
	if !c.Activa {
		return ErrCuentaInactiva
	}
	if err := validarMonto(m, canal, caj); err != nil {
		return err
	}
	acumulado := c.RetiradoHoy
	if c.FechaRetiro != hoy {
		acumulado = 0
	}
	if acumulado+m > c.LimiteDiario {
		return ErrLimiteDiario.Con("El monto excede el límite diario (%s); ya retiró %s hoy", c.LimiteDiario, acumulado)
	}
	if m > c.Saldo { // RN-01 y RN-02: sin sobregiro
		return ErrSaldoInsuficiente
	}
	if canal == CanalATM && m > caj.EfectivoDisponible {
		return ErrCajeroSinEfectivo
	}
	return nil
}

// Retirar aplica el retiro (previa validación).
func Retirar(c *Cuenta, caj *Cajero, m Centavos, canal Canal, hoy string) error {
	if err := PuedeRetirar(c, caj, m, canal, hoy); err != nil {
		return err
	}
	if c.FechaRetiro != hoy {
		c.FechaRetiro, c.RetiradoHoy = hoy, 0
	}
	c.RetiradoHoy += m
	c.Saldo -= m
	if canal == CanalATM {
		caj.EfectivoDisponible -= m
	}
	return nil
}

// PuedeAbonar valida el abono en efectivo (US-01b).
func PuedeAbonar(c *Cuenta, caj *Cajero, m Centavos, canal Canal) error {
	if canal == CanalATM && !caj.Operativo {
		return ErrCajeroInoperativo
	}
	if !c.Activa {
		return ErrCuentaInactiva
	}
	return validarMonto(m, canal, caj)
}

// Abonar aplica el abono (previa validación).
func Abonar(c *Cuenta, caj *Cajero, m Centavos, canal Canal) error {
	if err := PuedeAbonar(c, caj, m, canal); err != nil {
		return err
	}
	c.Saldo += m
	if canal == CanalATM {
		caj.EfectivoDisponible += m
	}
	return nil
}
