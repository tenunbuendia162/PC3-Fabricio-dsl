// Package repository persiste el estado del banco. La implementación JSON
// escribe en archivo temporal + rename, de modo que el guardado es atómico:
// o se escribe el estado completo (saldo + log) o no se escribe nada (RN-03).
package repository

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"prj_withdrawal/internal/domain"
)

// Store es el puerto de persistencia.
type Store interface {
	Load() (*domain.Banco, error)
	Save(*domain.Banco) error
}

// JSONStore guarda el estado en un archivo JSON legible (fuente de datos + log).
type JSONStore struct {
	Path string
	mu   sync.Mutex
}

func NewJSONStore(path string) *JSONStore { return &JSONStore{Path: path} }

// Load lee el archivo; si no existe crea los datos semilla y los guarda.
func (s *JSONStore) Load() (*domain.Banco, error) {
	s.mu.Lock()
	b, err := os.ReadFile(s.Path)
	s.mu.Unlock()
	if errors.Is(err, os.ErrNotExist) {
		seed := domain.Seed()
		return seed, s.Save(seed)
	}
	if err != nil {
		return nil, err
	}
	var banco domain.Banco
	if err := json.Unmarshal(b, &banco); err != nil {
		return nil, err
	}
	return &banco, nil
}

// Save escribe de forma atómica (tmp + rename).
func (s *JSONStore) Save(b *domain.Banco) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.Path), ".banco-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.Path)
}

// MemoryStore sirve para pruebas y para el modo dinámico por flags (sin tocar disco).
// Si FailSave es true, Save falla: permite probar la atomicidad.
type MemoryStore struct {
	Banco    *domain.Banco
	FailSave bool
}

func NewMemoryStore(b *domain.Banco) *MemoryStore { return &MemoryStore{Banco: b} }

func (m *MemoryStore) Load() (*domain.Banco, error) { return m.Banco.Clone(), nil }

func (m *MemoryStore) Save(b *domain.Banco) error {
	if m.FailSave {
		return errors.New("fallo simulado de persistencia")
	}
	m.Banco = b.Clone()
	return nil
}
