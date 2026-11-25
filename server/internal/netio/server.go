// Package netio maneja las conexiones WebSocket y traduce frames en mensajes
// del protocolo. En F0 solo implementa el intercambio ping/pong; el game loop
// autoritativo y el mundo llegan en F1 (ver docs/03-arquitectura.md).
package netio

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"munext/server/internal/protocol"
)

const (
	// writeWait es el tiempo máximo para escribir un mensaje.
	writeWait = 5 * time.Second
	// pongWait es el tiempo máximo sin recibir nada del cliente.
	// El cliente manda heartbeat/ping cada 2 s; si calla 20 s, se cierra.
	pongWait = 20 * time.Second
)

// Server expone el endpoint WebSocket del juego.
type Server struct {
	log      *slog.Logger
	upgrader websocket.Upgrader

	mu    sync.Mutex
	conns map[*websocket.Conn]struct{}
}

// New crea un Server con el logger dado.
func New(log *slog.Logger) *Server {
	return &Server{
		log: log,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			// En desarrollo aceptamos cualquier origen. En producción va detrás
			// de Caddy, que restringe el Origin (ver docs/06-seguridad.md).
			CheckOrigin: func(*http.Request) bool { return true },
		},
		conns: make(map[*websocket.Conn]struct{}),
	}
}

// Handler devuelve el mux HTTP del servidor de juego.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /ws", s.handleWS)
	return mux
}

// Close cierra todas las conexiones WebSocket activas. Es necesario en el
// apagado limpio: http.Server.Shutdown no cierra conexiones "hijacked".
func (s *Server) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for conn := range s.conns {
		_ = conn.Close()
	}
}

func (s *Server) track(conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conns[conn] = struct{}{}
}

func (s *Server) untrack(conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conns, conn)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		// El upgrade ya escribió una respuesta HTTP de error.
		s.log.Warn("upgrade websocket falló", "err", err, "remote", r.RemoteAddr)
		return
	}
	defer conn.Close()
	s.track(conn)
	defer s.untrack(conn)

	log := s.log.With("remote", r.RemoteAddr)
	log.Info("cliente conectado")
	defer log.Info("cliente desconectado")

	conn.SetReadLimit(protocol.MaxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Warn("lectura terminada con error", "err", err)
			}
			return
		}

		env, err := protocol.Decode(data)
		if err != nil {
			log.Warn("mensaje inválido: cerrando conexión", "err", err)
			_ = conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "mensaje inválido"),
				time.Now().Add(writeWait),
			)
			return
		}

		switch env.T {
		case "ping":
			pong, err := protocol.Encode("pong", map[string]any{
				"server_time_ms": time.Now().UnixMilli(),
			})
			if err != nil {
				log.Error("no se pudo codificar el pong", "err", err)
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.TextMessage, pong); err != nil {
				log.Warn("fallo al escribir el pong", "err", err)
				return
			}
		default:
			log.Debug("mensaje sin handler", "tipo", env.T)
		}
	}
}
