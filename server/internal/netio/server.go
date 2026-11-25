// Package netio maneja las conexiones WebSocket y traduce frames en comandos
// para el game loop (ver docs/03-arquitectura.md y docs/04-red.md).
package netio

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"munext/server/internal/accounts"
	"munext/server/internal/model"
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
	// dbTimeout limita las operaciones de cuentas por mensaje.
	dbTimeout = 5 * time.Second
)

// Accounts es la capa de cuentas que consume la red (implementada por
// accounts.Service; los tests usan un fake).
type Accounts interface {
	Register(ctx context.Context, username, password string) (model.User, error)
	Login(ctx context.Context, username, password string) (model.User, error)
	ListCharacters(ctx context.Context, userID int64) ([]model.CharacterSummary, error)
	CreateCharacter(ctx context.Context, userID int64, name, class string) (model.CharacterSummary, error)
	DeleteCharacter(ctx context.Context, userID, characterID int64) error
	LoadCharacter(ctx context.Context, userID, characterID int64) (model.Character, error)
}

// Server expone el endpoint WebSocket del juego.
type Server struct {
	log      *slog.Logger
	inbox    chan<- world.Command
	accounts Accounts
	tokens   *tokenStore
	upgrader websocket.Upgrader

	mu    sync.Mutex
	conns map[*websocket.Conn]struct{}

	nextSession atomic.Int64
}

// New crea un Server que envía los comandos de los clientes al mundo.
func New(log *slog.Logger, inbox chan<- world.Command, accountsSvc Accounts) *Server {
	return &Server{
		log:      log,
		inbox:    inbox,
		accounts: accountsSvc,
		tokens:   newTokenStore(),
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

// connSession es el estado de autenticación de una conexión.
type connSession struct {
	userID   int64
	username string
	token    string
	inWorld  bool
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

	sess := &connSession{}
	defer func() {
		if sess.inWorld {
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

		case "register":
			if sess.userID != 0 {
				s.sendError(out, "already_logged", "Ya estás autenticado")
				continue
			}
			req, ok := decodeAuth(log, env)
			if !ok {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
			user, err := s.accounts.Register(ctx, req.User, req.Pass)
			cancel()
			if err != nil {
				s.sendAccountError(out, log, err)
				continue
			}
			token, err := s.tokens.issue(user)
			if err != nil {
				log.Error("no se pudo emitir token", "err", err)
				s.sendError(out, "internal", "Error interno del servidor")
				continue
			}
			if !s.sendLoginOK(out, user, token, log) {
				continue
			}
			sess.userID, sess.username, sess.token = user.ID, user.Username, token
			log = log.With("user", user.Username)
			log.Info("registro y login", "user_id", user.ID)

		case "login":
			if sess.userID != 0 {
				s.sendError(out, "already_logged", "Ya estás autenticado")
				continue
			}
			req, ok := decodeAuth(log, env)
			if !ok {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
			user, err := s.accounts.Login(ctx, req.User, req.Pass)
			cancel()
			if err != nil {
				s.sendAccountError(out, log, err)
				continue
			}
			token, err := s.tokens.issue(user)
			if err != nil {
				log.Error("no se pudo emitir token", "err", err)
				s.sendError(out, "internal", "Error interno del servidor")
				continue
			}
			if !s.sendLoginOK(out, user, token, log) {
				continue
			}
			sess.userID, sess.username, sess.token = user.ID, user.Username, token
			log = log.With("user", user.Username)
			log.Info("login", "user_id", user.ID)

		case "resume":
			if sess.userID != 0 {
				s.sendError(out, "already_logged", "Ya estás autenticado")
				continue
			}
			var req protocol.Resume
			if err := json.Unmarshal(env.D, &req); err != nil {
				log.Warn("resume malformado", "err", err)
				return
			}
			info, ok := s.tokens.lookup(req.Token)
			if !ok {
				s.sendError(out, "session_expired", "La sesión expiró; iniciá sesión de nuevo")
				continue
			}
			user := model.User{ID: info.UserID, Username: info.Username}
			if !s.sendLoginOK(out, user, req.Token, log) {
				continue
			}
			sess.userID, sess.username, sess.token = user.ID, user.Username, req.Token
			log = log.With("user", user.Username)
			log.Info("sesión reanudada", "user_id", user.ID)

		case "create_char":
			if !requireAuth(sess, out, s) {
				continue
			}
			var req protocol.CreateChar
			if err := json.Unmarshal(env.D, &req); err != nil {
				log.Warn("create_char malformado", "err", err)
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
			char, err := s.accounts.CreateCharacter(ctx, sess.userID, req.Name, req.Class)
			cancel()
			if err != nil {
				s.sendAccountError(out, log, err)
				continue
			}
			data, err := protocol.Encode("char_created", protocol.CharacterCreated{Char: char})
			if err == nil {
				enqueue(out, data)
			}

		case "delete_char":
			if !requireAuth(sess, out, s) {
				continue
			}
			var req protocol.DeleteChar
			if err := json.Unmarshal(env.D, &req); err != nil {
				log.Warn("delete_char malformado", "err", err)
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
			err := s.accounts.DeleteCharacter(ctx, sess.userID, req.CharID)
			cancel()
			if err != nil {
				s.sendAccountError(out, log, err)
				continue
			}
			data, err := protocol.Encode("char_deleted", protocol.CharacterDeleted{CharID: req.CharID})
			if err == nil {
				enqueue(out, data)
			}

		case "select_char":
			if !requireAuth(sess, out, s) {
				continue
			}
			if sess.inWorld {
				s.sendError(out, "already_in_world", "Ya estás en el mundo")
				continue
			}
			var req protocol.SelectChar
			if err := json.Unmarshal(env.D, &req); err != nil {
				log.Warn("select_char malformado", "err", err)
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
			char, err := s.accounts.LoadCharacter(ctx, sess.userID, req.CharID)
			cancel()
			if err != nil {
				s.sendAccountError(out, log, err)
				continue
			}
			s.inbox <- world.Join{SessionID: sessionID, Character: char, Out: out}
			sess.inWorld = true
			log.Info("entrando al mundo", "character", char.Name, "character_id", char.ID)

		case "move_req":
			if !sess.inWorld {
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

func decodeAuth(log *slog.Logger, env protocol.Envelope) (protocol.Auth, bool) {
	var req protocol.Auth
	if err := json.Unmarshal(env.D, &req); err != nil {
		log.Warn("credenciales malformadas", "err", err)
		return req, false
	}
	return req, true
}

func requireAuth(sess *connSession, out chan<- []byte, s *Server) bool {
	if sess.userID == 0 {
		s.sendError(out, "auth_required", "Iniciá sesión primero")
		return false
	}
	return true
}

// sendLoginOK lista personajes y responde login_ok. Devuelve false si falló
// (ya envió el error correspondiente).
func (s *Server) sendLoginOK(out chan<- []byte, user model.User, token string, log *slog.Logger) bool {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	chars, err := s.accounts.ListCharacters(ctx, user.ID)
	if err != nil {
		s.sendAccountError(out, log, err)
		return false
	}
	if chars == nil {
		chars = []model.CharacterSummary{}
	}

	data, err := protocol.Encode("login_ok", protocol.LoginOK{
		Token: token,
		User:  user.Username,
		Chars: chars,
	})
	if err != nil {
		log.Error("no se pudo codificar login_ok", "err", err)
		return false
	}
	enqueue(out, data)
	return true
}

func (s *Server) sendAccountError(out chan<- []byte, log *slog.Logger, err error) {
	switch {
	case errors.Is(err, accounts.ErrInvalidCredentials):
		s.sendError(out, "auth_failed", "Usuario o contraseña incorrectos")
	case errors.Is(err, accounts.ErrInvalidUsername):
		s.sendError(out, "bad_username", "Usuario inválido: 3-16 caracteres (letras, números, _ o -)")
	case errors.Is(err, accounts.ErrWeakPassword):
		s.sendError(out, "weak_password", "La contraseña debe tener entre 6 y 72 caracteres")
	case errors.Is(err, accounts.ErrInvalidCharacterName):
		s.sendError(out, "bad_char_name", "Nombre inválido: 3-16 letras")
	case errors.Is(err, accounts.ErrInvalidClass):
		s.sendError(out, "bad_class", "Clase inválida")
	case errors.Is(err, model.ErrUsernameTaken):
		s.sendError(out, "user_taken", "Ese nombre de usuario ya existe")
	case errors.Is(err, model.ErrCharacterNameTaken):
		s.sendError(out, "char_name_taken", "Ese nombre de personaje ya existe")
	case errors.Is(err, model.ErrCharacterLimit):
		s.sendError(out, "char_limit", "Máximo 3 personajes por cuenta")
	case errors.Is(err, model.ErrNotFound):
		s.sendError(out, "not_found", "No encontrado")
	default:
		log.Error("error de cuentas", "err", err)
		s.sendError(out, "internal", "Error interno del servidor")
	}
}

func (s *Server) sendError(out chan<- []byte, code, msg string) {
	data, err := protocol.Encode("err", protocol.Error{Code: code, Msg: msg})
	if err == nil {
		enqueue(out, data)
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

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// --- tokens de sesión ---

type tokenInfo struct {
	UserID   int64
	Username string
}

type tokenStore struct {
	mu     sync.Mutex
	tokens map[string]tokenInfo
}

func newTokenStore() *tokenStore {
	return &tokenStore{tokens: make(map[string]tokenInfo)}
}

func (t *tokenStore) issue(user model.User) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	t.mu.Lock()
	defer t.mu.Unlock()
	t.tokens[token] = tokenInfo{UserID: user.ID, Username: user.Username}
	return token, nil
}

func (t *tokenStore) lookup(token string) (tokenInfo, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	info, ok := t.tokens[token]
	return info, ok
}
