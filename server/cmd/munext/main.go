// Command munext arranca el servidor de juego autoritativo.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"munext/server/internal/accounts"
	"munext/server/internal/db"
	"munext/server/internal/gamedata"
	"munext/server/internal/model"
	"munext/server/internal/netio"
	"munext/server/internal/world"
)

const defaultDSN = "postgres://postgres:postgres@127.0.0.1:5432/munext?sslmode=disable"

func main() {
	dsnDefault := os.Getenv("MUNEXT_DATABASE_URL")
	if dsnDefault == "" {
		dsnDefault = defaultDSN
	}

	addr := flag.String("addr", "127.0.0.1:8080", "dirección de escucha (host:puerto)")
	dsn := flag.String("dsn", dsnDefault, "URL de PostgreSQL (o MUNEXT_DATABASE_URL)")
	gamedataPath := flag.String("gamedata", "../shared/gamedata", "directorio de datos estáticos")
	verbose := flag.Bool("v", false, "log detallado (nivel debug)")
	flag.Parse()

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- Datos estáticos del juego ---
	dataDir, err := resolveGameDataPath(*gamedataPath)
	if err != nil {
		logger.Error("no se pudo encontrar gamedata", "err", err)
		os.Exit(1)
	}
	gameData, err := gamedata.Load(os.DirFS(dataDir), "items.json", "mobs.json", "maps/llanuras.json")
	if err != nil {
		logger.Error("gamedata inválida", "err", err)
		os.Exit(1)
	}
	logger.Info("gamedata cargada", "dir", dataDir, "map", gameData.MapID,
		"mobs", len(gameData.Mobs), "items", len(gameData.Items), "spawns", len(gameData.Spawns))

	// --- Base de datos ---
	pool, err := db.Open(ctx, *dsn)
	if err != nil {
		logger.Error("no se pudo conectar a la base de datos (¿corriste `make devdb`?)", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, *dsn, logger); err != nil {
		logger.Error("no se pudieron aplicar las migraciones", "err", err)
		os.Exit(1)
	}
	logger.Info("base de datos lista")

	store := db.NewStore(pool)
	accountsSvc := accounts.New(store)

	// --- Persistencia asíncrona ---
	saveCh := make(chan model.CharacterState, 256)
	saverDone := make(chan struct{})
	go func() {
		defer close(saverDone)
		db.RunSaver(saveCh, store, logger)
	}()

	// --- Mundo + red ---
	runCtx, cancelWorld := context.WithCancel(context.Background())
	defer cancelWorld()

	gameWorld := world.New(logger, saveCh, gameData)
	worldDone := make(chan struct{})
	go func() {
		defer close(worldDone)
		gameWorld.Run(runCtx)
	}()

	gameServer := netio.New(logger, gameWorld.Inbox(), accountsSvc)
	srv := &http.Server{
		Addr:              *addr,
		Handler:           gameServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("servidor escuchando", "addr", *addr, "ws", "ws://"+*addr+"/ws")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("el servidor terminó con error", "err", err)
			stop()
		}
	}()

	<-ctx.Done()

	// Apagado ordenado: conexiones → mundo (guarda a todos) → saver → pool.
	logger.Info("apagando servidor...")
	gameServer.Close()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("apagado forzado del HTTP server", "err", err)
	}

	cancelWorld()
	select {
	case <-worldDone:
	case <-time.After(5 * time.Second):
		logger.Warn("el mundo no terminó a tiempo")
	}

	close(saveCh)
	select {
	case <-saverDone:
	case <-time.After(10 * time.Second):
		logger.Warn("el saver no terminó a tiempo")
	}

	logger.Info("servidor detenido")
}

// resolveGameDataPath prueba la ruta pedida y dos ubicaciones habituales.
func resolveGameDataPath(preferred string) (string, error) {
	candidates := []string{preferred, "shared/gamedata", "../shared/gamedata"}
	for _, dir := range candidates {
		if dir == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "mobs.json")); err == nil {
			return dir, nil
		}
	}
	return "", fmt.Errorf("no se encontró mobs.json (probé %q, shared/gamedata, ../shared/gamedata)", preferred)
}
