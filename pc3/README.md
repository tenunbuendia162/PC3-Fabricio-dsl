# prj_withdrawal · Cajero Automático (Retiro, Abono y Transacción)

Proyecto del curso **DSL / Gherkin-BDD** (UNI · FIIS · Escuela de Ingeniería de Software), construido sobre el
codeLAB-02 (`Feature: Withdrawing cash`). Desarrollo **iterativo e incremental** en Go, sin dependencias externas.

| Acápite | Entregable | Dónde está |
|---|---|---|
| 1 | Base: Retiro + Gherkin + CLI por flags (`-nombre -saldo -retiro -test`) | `features/retiro-cash.feature`, `cmd/cli` (modo dinámico) |
| 2 | **US-01b** Abono en efectivo | `features/abono-cash.feature`, `domain.Abonar` |
| 3 | **US-04** Integridad de la transacción + log JSON (éxito, fallo, error) | `features/transaccion.feature`, `service.operar`, `data/banco.json` |
| 4.a | Refactorización a **App Web** (API REST + UI), lista para GitHub | `cmd/web`, `internal/web`, `Dockerfile`, `.github/workflows` |

## Requisitos
Go 1.22 o superior. No hay que descargar módulos.

## Uso rápido
```bash
go test ./...                 # pruebas unitarias + 25 escenarios Gherkin
go run ./cmd/cli              # CLI interactivo (cajero por terminal)
go run ./cmd/cli -bdd         # ejecuta los .feature mostrando Dado/Cuando/Entonces
go run ./cmd/web              # App Web en http://localhost:8080
```
Cuentas demo (PIN): `1001` Alice (1234) · `1002` Hamza (4321) · `1003` Carlos (1111) · `1004` María (2222) · `1005` Pedro (3333).

### Modo dinámico (compatible con el codeLAB-02)
```bash
go run ./cmd/cli -nombre Carlos -saldo 500 -retiro 150          # automático -> Escenario 1 (éxito)
go run ./cmd/cli -nombre Maria  -saldo 50  -retiro 100 -test fallo
go run ./cmd/cli -nombre Pedro  -saldo 300 -retiro -20 -test error
go run ./cmd/cli -nombre Ana    -saldo 100 -abono 50            # abono en efectivo
go run ./cmd/cli -h
```
En Windows: `go build -o prj_retiro.exe ./cmd/cli`.

## Arquitectura (refactorizada por capas)
```
cmd/cli, cmd/web      → puntos de entrada (solo cablean dependencias)
internal/cli, web     → adaptadores: terminal / HTTP+UI  (sin reglas de negocio)
internal/service      → casos de uso atómicos (commit/rollback) + historial
internal/repository   → Store (JSON atómico: tmp+rename | memoria)
internal/domain       → Cuenta, Cajero, Transaccion, reglas RN-01..RN-03, dinero en centavos
internal/gherkin      → intérprete de .feature (es/en, Esquema del escenario, Ejemplos)
internal/bdd          → step definitions: la especificación ES la prueba
features/*.feature    → especificación ejecutable
```
Decisiones clave: dinero como **entero en centavos** (sin errores de redondeo); validar **antes** de mutar;
operar sobre una **copia** del estado y guardar saldo + log en **una sola escritura atómica** (RN-03).

## Reglas de negocio
- **RN-01** No se retira más que el saldo disponible. **RN-02** Sin saldos negativos (sin sobregiro).
- **RN-03** Transacciones atómicas: o se completa todo o no se hace nada.
- Validaciones (US-03): monto > 0, múltiplo de billetes (solo canal ATM), límite diario acumulado.
- Estados del log: `EXITOSA` · `RECHAZADA` (regla de negocio: saldo, efectivo del cajero) · `ERROR` (dato inválido, límite, sistema).

## API REST (App Web)
| Método | Ruta | Descripción |
|---|---|---|
| POST | `/api/login` `{cuenta,pin}` | devuelve `token` (Bearer, 15 min; bloqueo tras 3 fallos) |
| GET | `/api/cuenta` | saldo y límites |
| POST | `/api/retiro` `{monto}` | 200 éxito · 422 rechazada/error (queda registrada) · 500 fallo de sistema |
| POST | `/api/abono` `{monto}` | ídem para abono |
| GET | `/api/transacciones` | log en JSON |
| GET | `/healthz` | salud |

## Publicar en GitHub
```bash
git init && git add . && git commit -m "feat: cajero BDD (retiro, abono, transacción) + app web"
git branch -M main
git remote add origin https://github.com/<usuario>/prj_withdrawal.git
git push -u origin main
```
El workflow `.github/workflows/ci.yml` corre `go vet`, `go test -race` y los escenarios BDD en cada push.

## Limitaciones conocidas (alcance académico)
PIN con SHA-256 sin sal por usuario más allá del ID de cuenta (en producción: bcrypt/argon2); sesiones en memoria;
un solo archivo JSON como base de datos (en producción: base transaccional). Un monto no numérico (“abc”) se rechaza
sin registrarse, porque no puede representarse como transacción.
