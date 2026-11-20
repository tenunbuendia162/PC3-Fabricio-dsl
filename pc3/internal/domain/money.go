// Package domain contiene el modelo de dominio (Cuenta, Cajero, Transaccion)
// y las reglas de negocio puras RN-01, RN-02, RN-03 y validaciones de monto.
package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Centavos representa dinero como entero (unidad mínima) para evitar
// errores de redondeo ("Los montos cast(entero, monto)" del modelo OCL).
type Centavos int64

// ParseMonto convierte "200", "200.5", "-20.00" o "S/ 234.56" a Centavos.
// Rechaza texto no numérico y más de 2 decimales.
func ParseMonto(s string) (Centavos, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimSpace(strings.TrimPrefix(strings.ToUpper(s), "S/"))
	if s == "" {
		return 0, errors.New("monto vacío")
	}
	neg := false
	switch s[0] {
	case '-':
		neg, s = true, s[1:]
	case '+':
		s = s[1:]
	}
	ent, frac, hasDot := strings.Cut(s, ".")
	if ent == "" && (!hasDot || frac == "") {
		return 0, errors.New("monto no numérico")
	}
	if len(frac) > 2 {
		return 0, errors.New("máximo 2 decimales")
	}
	for len(frac) < 2 {
		frac += "0"
	}
	var v int64
	for _, r := range ent + frac {
		if r < '0' || r > '9' {
			return 0, errors.New("monto no numérico")
		}
		if v > (1<<62)/10 {
			return 0, errors.New("monto demasiado grande")
		}
		v = v*10 + int64(r-'0')
	}
	if neg {
		v = -v
	}
	return Centavos(v), nil
}

// String formatea con 2 decimales: 23456 -> "234.56".
func (c Centavos) String() string {
	sign := ""
	v := int64(c)
	if v < 0 {
		sign, v = "-", -v
	}
	return fmt.Sprintf("%s%d.%02d", sign, v/100, v%100)
}

// MarshalJSON escribe el monto como número decimal exacto (234.56).
func (c Centavos) MarshalJSON() ([]byte, error) { return []byte(c.String()), nil }

// UnmarshalJSON acepta número (234.56) o cadena ("234.56").
func (c *Centavos) UnmarshalJSON(b []byte) error {
	v, err := ParseMonto(strings.Trim(string(b), `"`))
	if err != nil {
		return err
	}
	*c = v
	return nil
}
