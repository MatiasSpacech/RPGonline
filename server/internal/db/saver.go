package db

import (
	"context"
	"log/slog"
	"time"

	"munext/server/internal/model"
)

// RunSaver consume la cola de guardado en su propia goroutine. El game loop
// nunca espera a la base de datos (ver docs/03-arquitectura.md).
func RunSaver(saveCh <-chan model.CharacterState, store *Store, log *slog.Logger) {
	for state := range saveCh {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := store.SaveCharacterState(ctx, state); err != nil {
			log.Error("no se pudo guardar el personaje",
				"character_id", state.CharacterID, "err", err)
		}
		cancel()
	}
	log.Info("saver de personajes detenido")
}
