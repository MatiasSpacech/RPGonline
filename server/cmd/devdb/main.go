// Command devdb corre PostgreSQL embebido para desarrollo local, sin Docker
// ni permisos de administrador. Descarga binarios oficiales (Zonky) la
// primera vez y los cachea en el home del usuario.
//
// Uso:
//
//	go run ./cmd/devdb            # postgres://postgres:postgres@127.0.0.1:5432/munext
//	go run ./cmd/devdb -port 5433
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
)

func main() {
	dataDir := flag.String("data", ".data/postgres", "directorio de datos (persistente)")
	port := flag.Uint("port", 5432, "puerto TCP de Postgres")
	flag.Parse()

	abs, err := filepath.Abs(*dataDir)
	if err != nil {
		log.Fatalf("ruta de datos inválida: %v", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		log.Fatalf("no se pudo crear %s: %v", abs, err)
	}

	cfg := embeddedpostgres.DefaultConfig().
		Version(embeddedpostgres.V16).
		Username("postgres").
		Password("postgres").
		Database("munext").
		Port(uint32(*port)).
		DataPath(filepath.Join(abs, "data")).
		RuntimePath(filepath.Join(abs, "runtime")).
		StartTimeout(90 * time.Second).
		Logger(os.Stdout)

	db := embeddedpostgres.NewDatabase(cfg)
	log.Printf("iniciando Postgres embebido v16 (datos: %s, puerto: %d)...", abs, *port)
	if err := db.Start(); err != nil {
		log.Fatalf("no se pudo iniciar Postgres embebido: %v", err)
	}
	log.Printf("Postgres listo: postgres://postgres:postgres@127.0.0.1:%d/munext?sslmode=disable", *port)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("deteniendo Postgres embebido...")
	if err := db.Stop(); err != nil {
		log.Printf("error al detener Postgres: %v", err)
	}
}
