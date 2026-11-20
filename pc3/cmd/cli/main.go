// Comando cli: cajero automático interactivo (acápites 1-3).
//
//	go run ./cmd/cli                                    # modo interactivo
//	go run ./cmd/cli -bdd                               # ejecuta los .feature y sale
//	go run ./cmd/cli -nombre Carlos -saldo 500 -retiro 150   # modo dinámico (codeLAB-02)
package main

import (
	"flag"
	"fmt"
	"os"

	"prj_withdrawal/internal/bdd"
	"prj_withdrawal/internal/cli"
	"prj_withdrawal/internal/domain"
	"prj_withdrawal/internal/gherkin"
	"prj_withdrawal/internal/repository"
	"prj_withdrawal/internal/service"
)

func main() {
	data := flag.String("data", "data/banco.json", "archivo JSON de datos y log de transacciones")
	features := flag.String("features", "features", "carpeta con los archivos .feature")
	runBDD := flag.Bool("bdd", false, "ejecutar los escenarios Gherkin y salir")
	nombre := flag.String("nombre", "", "modo dinámico: nombre del titular")
	saldo := flag.String("saldo", "0", "modo dinámico: saldo inicial")
	retiro := flag.String("retiro", "0", "modo dinámico: cantidad a retirar")
	abono := flag.String("abono", "", "modo dinámico: cantidad a abonar")
	test := flag.String("test", "", "modo dinámico: exito | fallo | error | abono | abono-error")
	flag.Parse()

	if *runBDD {
		res, err := bdd.Run(*features)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(2)
		}
		if _, failed := gherkin.Report(os.Stdout, res); failed > 0 {
			os.Exit(1)
		}
		return
	}

	if *nombre != "" {
		s, e1 := domain.ParseMonto(*saldo)
		r, e2 := domain.ParseMonto(*retiro)
		if e1 != nil || e2 != nil {
			fmt.Fprintln(os.Stderr, "error: -saldo y -retiro deben ser números (ej. 150 o 150.50)")
			os.Exit(2)
		}
		var ab *domain.Centavos
		if *abono != "" {
			v, err := domain.ParseMonto(*abono)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error: -abono debe ser un número")
				os.Exit(2)
			}
			ab = &v
		}
		if !cli.Dinamico(os.Stdout, *nombre, s, r, ab, *test) {
			os.Exit(1)
		}
		return
	}

	svc, err := service.New(repository.NewJSONStore(*data))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error al abrir los datos:", err)
		os.Exit(1)
	}
	cli.New(svc, os.Stdin, os.Stdout, *features).Run()
}
