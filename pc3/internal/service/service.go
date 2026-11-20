// Package service orquesta los casos de uso UC-001 (Retiro), US-01b (Abono)
// y US-04 (Integridad de la transacción): cada operación es atómica y deja
// rastro en el historial, sea EXITOSA, RECHAZADA o ERROR.
package service

import (
	"fmt"
	"sync"
	"time"

	"prj_withdrawal/internal/domain"
	"prj_withdrawal/internal/repository"
)

// Service es seguro para uso concurrente.
type Service struct {
	mu    sync.Mutex
	store repository.Store
	banco *domain.Banco
	Now   func() time.Time
}

// New carga el estado desde el store.
func New(store repository.Store) (*Service, error) {
	b, err := store.Load()
	if err != nil {
		return nil, err
	}
	return &Service{store: store, banco: b, Now: time.Now}, nil
}

// Vista es lo que se expone de una cuenta (sin hash de PIN).
type Vista struct {
	ID           string          `json:"id"`
	Titular      string          `json:"titular"`
	Saldo        domain.Centavos `json:"saldo"`
	Activa       bool            `json:"activa"`
	LimiteDiario domain.Centavos `json:"limite_diario"`
	RetiradoHoy  domain.Centavos `json:"retirado_hoy"`
}

func vista(c *domain.Cuenta, hoy string) Vista {
	ret := c.RetiradoHoy
	if c.FechaRetiro != hoy {
		ret = 0
	}
	return Vista{c.ID, c.Titular, c.Saldo, c.Activa, c.LimiteDiario, ret}
}

func (s *Service) hoy() string { return s.Now().Format("2006-01-02") }

// Autenticar valida cuenta + PIN (precondición del caso de uso).
func (s *Service) Autenticar(id, pin string) (Vista, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.banco.Cuenta(id)
	if c == nil || !c.PinValido(pin) {
		return Vista{}, domain.ErrAutenticacion // mismo mensaje: no revela si la cuenta existe
	}
	return vista(c, s.hoy()), nil
}

// Cuenta devuelve la vista actual de una cuenta.
func (s *Service) Cuenta(id string) (Vista, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.banco.Cuenta(id)
	if c == nil {
		return Vista{}, domain.ErrCuentaNoExiste
	}
	return vista(c, s.hoy()), nil
}

// Cajero devuelve el estado del cajero.
func (s *Service) Cajero() domain.Cajero {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.banco.Cajero
}

// Historial devuelve las transacciones de una cuenta (más antigua primero).
func (s *Service) Historial(id string) []domain.Transaccion {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []domain.Transaccion{}
	for _, t := range s.banco.Transacciones {
		if t.CuentaID == id {
			out = append(out, t)
		}
	}
	return out
}

// Retirar ejecuta UC-001. Siempre devuelve la transacción registrada; si err != nil
// la operación no se aplicó. Si ni siquiera pudo registrarse, tx.ID está vacío.
func (s *Service) Retirar(id string, m domain.Centavos, canal domain.Canal) (domain.Transaccion, error) {
	return s.operar(domain.Retiro, id, m, canal)
}

// Abonar ejecuta US-01b (abono en efectivo).
func (s *Service) Abonar(id string, m domain.Centavos, canal domain.Canal) (domain.Transaccion, error) {
	return s.operar(domain.Abono, id, m, canal)
}

// operar es el núcleo transaccional (US-04 / RN-03):
//  1. trabaja sobre una COPIA del estado;
//  2. aplica la regla (valida antes de mutar);
//  3. agrega la transacción al historial (éxito, rechazo o error);
//  4. guarda todo de una vez; si el guardado falla, la copia se descarta
//     (rollback) y el estado en memoria queda intacto.
func (s *Service) operar(tipo domain.Tipo, id string, m domain.Centavos, canal domain.Canal) (domain.Transaccion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.Now()
	work := s.banco.Clone()
	work.Secuencia++
	tx := domain.Transaccion{
		ID: fmt.Sprintf("TX-%06d", work.Secuencia), Fecha: now.UTC().Truncate(time.Second),
		Tipo: tipo, CuentaID: id, Canal: canal, Monto: m,
	}

	var opErr error
	if c := work.Cuenta(id); c == nil {
		opErr = domain.ErrCuentaNoExiste
	} else {
		tx.SaldoAntes = c.Saldo
		if tipo == domain.Retiro {
			opErr = domain.Retirar(c, &work.Cajero, m, canal, now.Format("2006-01-02"))
		} else {
			opErr = domain.Abonar(c, &work.Cajero, m, canal)
		}
		tx.SaldoDespues = c.Saldo
	}

	tx.Estado = domain.EstadoDe(opErr)
	if opErr == nil {
		tx.Mensaje = "Operación realizada con éxito"
	} else {
		tx.Mensaje = opErr.Error()
		if e, ok := opErr.(*domain.Err); ok {
			tx.Codigo = e.Codigo
		}
	}
	work.Transacciones = append(work.Transacciones, tx)

	if err := s.store.Save(work); err != nil {
		// Rollback: s.banco no se tocó. Nada se aplicó y nada quedó a medias.
		return domain.Transaccion{}, domain.ErrSistema.Con("%s (%v)", domain.ErrSistema.Mensaje, err)
	}
	s.banco = work // commit
	return tx, opErr
}
