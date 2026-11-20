// Package web expone el servicio bancario como API REST + interfaz web (acápite 4.a).
// La lógica de negocio NO está aquí: solo traduce HTTP <-> servicio (refactorización por capas).
package web

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"prj_withdrawal/internal/domain"
	"prj_withdrawal/internal/service"
)

//go:embed static
var staticFS embed.FS

const sesionTTL = 15 * time.Minute

type sesion struct {
	cuenta string
	expira time.Time
}

type Server struct {
	svc      *service.Service
	mu       sync.Mutex
	sesiones map[string]sesion
	fallos   map[string]int // intentos fallidos de login por cuenta
	Now      func() time.Time
}

func New(svc *service.Service) *Server {
	return &Server{svc: svc, sesiones: map[string]sesion{}, fallos: map[string]int{}, Now: time.Now}
}

// Handler devuelve el enrutador HTTP.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"estado": "ok"}) })
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("POST /api/logout", s.auth(s.logout))
	mux.HandleFunc("GET /api/cuenta", s.auth(s.cuenta))
	mux.HandleFunc("POST /api/retiro", s.auth(s.operacion(domain.Retiro)))
	mux.HandleFunc("POST /api/abono", s.auth(s.operacion(domain.Abono)))
	mux.HandleFunc("GET /api/transacciones", s.auth(s.transacciones))
	sub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/", http.FileServer(http.FS(sub)))
	return securityHeaders(mux)
}

func securityHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}

// ---- helpers ---------------------------------------------------------------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

type apiError struct {
	Codigo  string `json:"codigo"`
	Mensaje string `json:"mensaje"`
}

func writeErr(w http.ResponseWriter, code int, c, msg string) {
	writeJSON(w, code, map[string]any{"error": apiError{c, msg}})
}

func newToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type handlerCuenta func(w http.ResponseWriter, r *http.Request, cuenta, token string)

func (s *Server) auth(h handlerCuenta) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		s.mu.Lock()
		ses, ok := s.sesiones[token]
		if ok && s.Now().After(ses.expira) {
			delete(s.sesiones, token)
			ok = false
		}
		s.mu.Unlock()
		if !ok {
			writeErr(w, http.StatusUnauthorized, "E-AUTH", "Sesión no iniciada o expirada")
			return
		}
		h(w, r, ses.cuenta, token)
	}
}

// ---- handlers ----------------------------------------------------------------

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in struct{ Cuenta, Pin string }
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&in); err != nil {
		writeErr(w, 400, "E-JSON", "Solicitud inválida")
		return
	}
	s.mu.Lock()
	bloqueada := s.fallos[in.Cuenta] >= 3
	s.mu.Unlock()
	if bloqueada {
		writeErr(w, http.StatusTooManyRequests, "E-BLOQUEO", "Cuenta bloqueada temporalmente por intentos fallidos")
		return
	}
	v, err := s.svc.Autenticar(in.Cuenta, in.Pin)
	if err != nil {
		s.mu.Lock()
		s.fallos[in.Cuenta]++
		s.mu.Unlock()
		writeErr(w, http.StatusUnauthorized, "E-AUTH", err.Error())
		return
	}
	tok := newToken()
	s.mu.Lock()
	delete(s.fallos, in.Cuenta)
	s.sesiones[tok] = sesion{v.ID, s.Now().Add(sesionTTL)}
	s.mu.Unlock()
	writeJSON(w, 200, map[string]any{"token": tok, "cuenta": v})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request, _, token string) {
	s.mu.Lock()
	delete(s.sesiones, token)
	s.mu.Unlock()
	writeJSON(w, 200, map[string]string{"estado": "sesión cerrada"})
}

func (s *Server) cuenta(w http.ResponseWriter, r *http.Request, id, _ string) {
	v, err := s.svc.Cuenta(id)
	if err != nil {
		writeErr(w, 404, "E-NOEXISTE", err.Error())
		return
	}
	writeJSON(w, 200, v)
}

func (s *Server) transacciones(w http.ResponseWriter, r *http.Request, id, _ string) {
	writeJSON(w, 200, s.svc.Historial(id))
}

// operacion: 200 si se aplicó; 422 si fue RECHAZADA/ERROR (siempre con la transacción registrada).
func (s *Server) operacion(tipo domain.Tipo) handlerCuenta {
	return func(w http.ResponseWriter, r *http.Request, id, _ string) {
		var in struct {
			Monto domain.Centavos `json:"monto"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&in); err != nil {
			writeErr(w, 400, "E-MONTO", "Monto no numérico. Use un número como 150 o 150.50")
			return
		}
		var tx domain.Transaccion
		var err error
		if tipo == domain.Retiro {
			tx, err = s.svc.Retirar(id, in.Monto, domain.CanalWeb)
		} else {
			tx, err = s.svc.Abonar(id, in.Monto, domain.CanalWeb)
		}
		v, _ := s.svc.Cuenta(id)
		if err == nil {
			writeJSON(w, 200, map[string]any{"transaccion": tx, "cuenta": v})
			return
		}
		var de *domain.Err
		code := http.StatusUnprocessableEntity
		if errors.As(err, &de) && de.Codigo == domain.ErrSistema.Codigo {
			code = http.StatusInternalServerError
		}
		resp := map[string]any{"error": apiError{de.Codigo, de.Mensaje}, "cuenta": v}
		if tx.ID != "" {
			resp["transaccion"] = tx
		}
		writeJSON(w, code, resp)
	}
}
