// Package netio maneja las conexiones WebSocket y traduce frames en comandos
// para el game loop (ver docs/03-arquitectura.md y docs/04-red.md).
package netio

import (
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"munext/server/internal/protocol"
	"munext/server/internal/world"
)

const (
	// writeWait es el tiempo máximo para escribir un mensaje.
	writeWait = 5 * time.Second
	// pongWait es el tiempo máximo sin recibir nada del cliente.
	// El cliente manda ping cada 2 s; si calla 20 s, se cierra.
	pongWait = 20 * time.Second
	// sendBuffer es cuántos mensajes pendientes tolera un cliente.
	sendBuffer = 64
)

var userPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{3,16}$`)

// Server expone el endpoint WebSocket del juego.
type Server struct {
	log      *slog.Logger
	inbox    chan<- world.Command
	upgrader websocket.Upgrader

	mu    sync.Mutex
	conns map[*websocket.Conn]struct{}

	nextSession atomic.Int64
}

// New crea un Server que envía los comandos de los clientes al mundo.
func New(log *slog.Logger, inbox chan<- world.Command) *Server {
	return &Server{
		log:   log,
		inbox: inbox,
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

	sessionID := s.nextSession.Add(1)
	log := s.log.With("remote", r.RemoteAddr, "session", sessionID)
	log.Info("cliente conectado")

	// Todas las escrituras pasan por writePump: gorilla permite un escritor y
	// un lector concurrentes, y así no hay escrituras simultáneas.
	out := make(chan []byte, sendBuffer)
	done := make(chan struct{})
	go writePump(conn, out, done, log)
	defer close(done)

	loggedIn := false
	defer func() {
		if loggedIn {
			s.inbox <- world.Leave{SessionID: sessionID}
		}
		log.Info("cliente desconectado")
	}()

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
			return
		}

		switch env.T {
		case "ping":
			pong, err := protocol.Encode("pong", map[string]any{
				"server_time_ms": time.Now().UnixMilli(),
			})
			if err == nil {
				enqueue(out, pong)
			}

		case "login":
			if loggedIn {
				continue
			}
			var req protocol.Login
			if err := json.Unmarshal(env.D, &req); err != nil {
				log.Warn("login malformado", "err", err)
				return
			}
			user := strings.TrimSpace(req.User)
			if !userPattern.MatchString(user) {
				log.Warn("login rechazado: nombre inválido", "user", req.User)
				enqueueError(out, "bad_login", "El nombre debe tener 3-16 caracteres: letras, números, _ o -")
				return
			}
			s.inbox <- world.Join{SessionID: sessionID, User: user, Out: out}
			loggedIn = true
			log = log.With("user", user)
			log.Info("login aceptado")

		case "move_req":
			if !loggedIn {
				continue
			}
			var req protocol.MoveReq
			if err := json.Unmarshal(env.D, &req); err != nil {
				log.Warn("move_req malformado", "err", err)
				return
			}
			if !isFinite(req.X) || !isFinite(req.Z) {
				log.Warn("move_req con coordenadas inválidas", "x", req.X, "z", req.Z)
				return
			}
			s.inbox <- world.Move{SessionID: sessionID, X: req.X, Z: req.Z}

		default:
			log.Debug("mensaje sin handler", "tipo", env.T)
		}
	}
}

func writePump(conn *websocket.Conn, out <-chan []byte, done <-chan struct{}, log *slog.Logger) {
	for {
		select {
		case <-done:
			// Vaciar lo pendiente (p. ej. un error de login) antes de cerrar.
			for {
				select {
				case data := <-out:
					_ = conn.SetWriteDeadline(time.Now().Add(200 * time.Millisecond))
					if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
						return
					}
				default:
					return
				}
			}
		case data := <-out:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Warn("fallo al escribir", "err", err)
				_ = conn.Close() // despierta al lector
				return
			}
		}
	}
}

func enqueue(out chan<- []byte, data []byte) {
	select {
	case out <- data:
	default:
		// Cliente lento: descartar antes que bloquear.
	}
}

func enqueueError(out chan<- []byte, code, msg string) {
	data, err := protocol.Encode("err", protocol.Error{Code: code, Msg: msg})
	if err == nil {
		enqueue(out, data)
	}
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
