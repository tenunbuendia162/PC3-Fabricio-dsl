// Comando web: App Web del cajero (acápite 4.a).
//
//	go run ./cmd/web -addr :8080
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"prj_withdrawal/internal/repository"
	"prj_withdrawal/internal/service"
	"prj_withdrawal/internal/web"
)

func main() {
	addr := flag.String("addr", envOr("ADDR", ":8080"), "dirección de escucha")
	data := flag.String("data", envOr("DATA_FILE", "data/banco.json"), "archivo JSON de datos y log")
	flag.Parse()

	svc, err := service.New(repository.NewJSONStore(*data))
	if err != nil {
		log.Fatalf("no se pudo abrir los datos: %v", err)
	}
	srv := &http.Server{
		Addr: *addr, Handler: web.New(svc).Handler(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second,
	}
	log.Printf("Cajero Web UNI escuchando en http://localhost%s (datos: %s)", *addr, *data)
	log.Fatal(srv.ListenAndServe())
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
