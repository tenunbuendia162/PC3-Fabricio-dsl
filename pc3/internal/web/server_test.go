package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"prj_withdrawal/internal/domain"
	"prj_withdrawal/internal/repository"
	"prj_withdrawal/internal/service"
)

func setup(t *testing.T) (*httptest.Server, *repository.MemoryStore) {
	t.Helper()
	st := repository.NewMemoryStore(domain.Seed())
	svc, _ := service.New(st)
	ts := httptest.NewServer(New(svc).Handler())
	t.Cleanup(ts.Close)
	return ts, st
}

func call(t *testing.T, ts *httptest.Server, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, ts.URL+path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func login(t *testing.T, ts *httptest.Server, cuenta, pin string) string {
	code, d := call(t, ts, "POST", "/api/login", "", map[string]string{"cuenta": cuenta, "pin": pin})
	if code != 200 {
		t.Fatalf("login %d %v", code, d)
	}
	return d["token"].(string)
}

func TestSinTokenNoAutoriza(t *testing.T) {
	ts, _ := setup(t)
	if code, _ := call(t, ts, "POST", "/api/retiro", "", map[string]any{"monto": 10}); code != 401 {
		t.Fatalf("code = %d", code)
	}
}

func TestRetiroExitosoRechazadoYError(t *testing.T) {
	ts, _ := setup(t)
	tok := login(t, ts, "1001", "1234")

	code, d := call(t, ts, "POST", "/api/retiro", tok, map[string]any{"monto": 200})
	if code != 200 || d["cuenta"].(map[string]any)["saldo"].(float64) != 34.56 {
		t.Fatalf("exitoso: %d %v", code, d)
	}
	code, d = call(t, ts, "POST", "/api/retiro", tok, map[string]any{"monto": "200.00"})
	if code != 422 || d["error"].(map[string]any)["codigo"] != "E-SALDO" || d["transaccion"].(map[string]any)["estado"] != "RECHAZADA" {
		t.Fatalf("rechazado: %d %v", code, d)
	}
	code, d = call(t, ts, "POST", "/api/retiro", tok, map[string]any{"monto": -20})
	if code != 422 || d["transaccion"].(map[string]any)["estado"] != "ERROR" {
		t.Fatalf("error: %d %v", code, d)
	}
	code, _ = call(t, ts, "POST", "/api/retiro", tok, map[string]any{"monto": "abc"})
	if code != 400 {
		t.Fatalf("monto no numérico: %d", code)
	}
	_, _ = call(t, ts, "GET", "/api/cuenta", tok, nil)
	req, _ := http.NewRequest("GET", ts.URL+"/api/transacciones", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	res, _ := http.DefaultClient.Do(req)
	var h []map[string]any
	json.NewDecoder(res.Body).Decode(&h)
	if len(h) != 3 {
		t.Fatalf("historial = %d (éxito, rechazo y error deben quedar registrados)", len(h))
	}
}

func TestAbono(t *testing.T) {
	ts, _ := setup(t)
	tok := login(t, ts, "1002", "4321")
	code, d := call(t, ts, "POST", "/api/abono", tok, map[string]any{"monto": 0.24})
	if code != 200 || d["cuenta"].(map[string]any)["saldo"].(float64) != 199 {
		t.Fatalf("%d %v", code, d)
	}
}

func TestFalloDePersistenciaDevuelve500YNoCambiaSaldo(t *testing.T) {
	ts, st := setup(t)
	tok := login(t, ts, "1001", "1234")
	st.FailSave = true
	code, d := call(t, ts, "POST", "/api/retiro", tok, map[string]any{"monto": 100})
	if code != 500 || d["error"].(map[string]any)["codigo"] != "E-SISTEMA" {
		t.Fatalf("%d %v", code, d)
	}
	if d["cuenta"].(map[string]any)["saldo"].(float64) != 234.56 {
		t.Fatal("el saldo cambió")
	}
}

func TestBloqueoTrasTresIntentos(t *testing.T) {
	ts, _ := setup(t)
	for i := 0; i < 3; i++ {
		if code, _ := call(t, ts, "POST", "/api/login", "", map[string]string{"cuenta": "1001", "pin": "0000"}); code != 401 {
			t.Fatalf("intento %d: %d", i, code)
		}
	}
	if code, _ := call(t, ts, "POST", "/api/login", "", map[string]string{"cuenta": "1001", "pin": "1234"}); code != 429 {
		t.Fatalf("debió bloquear: %d", code)
	}
}

func TestEstaticosYSalud(t *testing.T) {
	ts, _ := setup(t)
	for _, p := range []string{"/", "/app.js", "/style.css", "/healthz"} {
		res, err := http.Get(ts.URL + p)
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("%s -> %v %v", p, res, err)
		}
		if res.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("faltan cabeceras de seguridad")
		}
	}
}
