package domain

import "fmt"

// EstadoTx es el estado de una transacción (modelo: estado {EXITOSA, RECHAZADA}
// extendido con ERROR para el US-04: log de éxito, fallo y error).
type EstadoTx string

const (
	Exitosa   EstadoTx = "EXITOSA"   // la operación se aplicó
	Rechazada EstadoTx = "RECHAZADA" // una regla de negocio la impidió (saldo insuficiente, ATM sin efectivo)
	Error     EstadoTx = "ERROR"     // dato inválido, límite excedido o fallo de sistema
)

// Err es un error de dominio con código estable, mensaje al cliente y estado de auditoría.
type Err struct {
	Codigo  string
	Mensaje string
	Estado  EstadoTx
}

func (e *Err) Error() string { return e.Mensaje }

// Is permite errors.Is por código aunque el mensaje tenga detalle distinto.
func (e *Err) Is(t error) bool {
	o, ok := t.(*Err)
	return ok && o.Codigo == e.Codigo
}

// Con devuelve una copia del error con un mensaje más específico.
func (e *Err) Con(format string, a ...any) *Err {
	return &Err{Codigo: e.Codigo, Mensaje: fmt.Sprintf(format, a...), Estado: e.Estado}
}

var (
	ErrSaldoInsuficiente = &Err{"E-SALDO", "Saldo insuficiente. Intente con un monto menor", Rechazada} // RN-01 / RN-02
	ErrCajeroSinEfectivo = &Err{"E-CAJERO", "El cajero no dispone de efectivo suficiente", Rechazada}
	ErrCajeroInoperativo = &Err{"E-CAJERO-OFF", "El cajero no está operativo", Rechazada}
	ErrMonto             = &Err{"E-MONTO", "El monto debe ser mayor que cero", Error}
	ErrDenominacion      = &Err{"E-DENOM", "El monto debe ser múltiplo de la denominación disponible", Error}
	ErrLimiteDiario      = &Err{"E-LIMITE", "El monto excede el límite diario de retiro", Error}
	ErrCuentaInactiva    = &Err{"E-INACTIVA", "La cuenta no está activa", Error}
	ErrCuentaNoExiste    = &Err{"E-NOEXISTE", "La cuenta no existe", Error}
	ErrAutenticacion     = &Err{"E-AUTH", "Cuenta o PIN incorrectos", Error}
	ErrSistema           = &Err{"E-SISTEMA", "Error del sistema: la operación no se aplicó", Error} // RN-03
)

// EstadoDe traduce cualquier error a su estado de auditoría.
func EstadoDe(err error) EstadoTx {
	if err == nil {
		return Exitosa
	}
	if e, ok := err.(*Err); ok {
		return e.Estado
	}
	return Error
}
