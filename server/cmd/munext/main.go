// Command munext arranca el servidor de juego (F0: ping/pong por WebSocket).
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"munext/server/internal/netio"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "dirección de escucha (host:puerto)")
	verbose := flag.Bool("v", false, "log detallado (nivel debug)")
	flag.Parse()

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	gameServer := netio.New(logger)
	srv := &http.Server{
		Addr:              *addr,
		Handler:           gameServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("servidor escuchando", "addr", *addr, "ws", "ws://"+*addr+"/ws")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("el servidor terminó con error", "err", err)
			os.Exit(1)
		}
	}()

	// Apagado limpio: Ctrl+C o SIGTERM cierran las conexiones en curso.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	logger.Info("apagando servidor...")
	gameServer.Close()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("apagado forzado", "err", err)
	}
}
