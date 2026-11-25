class_name NetClient
extends Node
## Cliente WebSocket de F1: conecta, hace login con nombre fijo, envía
## move_req y emite welcome/snapshot/pong. Reintenta solo si se cae el server.

signal connected
signal disconnected(reason: String)
signal welcome_received(data: Dictionary)
signal snapshot_received(data: Dictionary)
signal pong_received(rtt_ms: int)

enum State { DISCONNECTED, CONNECTING, CONNECTED }

const WS_URL := "ws://127.0.0.1:8080/ws"
const PING_INTERVAL := 2.0
const RETRY_DELAY := 3.0

var _socket := WebSocketPeer.new()
var _state := State.DISCONNECTED
var _retry_left := 0.0
var _ping_left := 0.0
var _last_ping_msec := 0
var _user := ""


func _ready() -> void:
	_connect_to_server()


## Guarda el nombre y, si ya hay conexión, manda el login.
func login(user: String) -> void:
	_user = user
	if _state == State.CONNECTED:
		_send_login()


func send_move(point: Vector3) -> void:
	if _state != State.CONNECTED:
		return
	_send({"t": "move_req", "d": {"x": point.x, "y": 0.0, "z": point.z}})


func _process(delta: float) -> void:
	if _state == State.DISCONNECTED:
		_retry_left -= delta
		if _retry_left <= 0.0:
			_connect_to_server()
		return

	_socket.poll()

	if _state == State.CONNECTING:
		match _socket.get_ready_state():
			WebSocketPeer.STATE_OPEN:
				_on_open()
			WebSocketPeer.STATE_CLOSED:
				_schedule_retry("No se pudo conectar (¿está corriendo el servidor?)")
		return

	# Estado CONNECTED
	if _socket.get_ready_state() != WebSocketPeer.STATE_OPEN:
		_schedule_retry("Servidor desconectado")
		return

	while _socket.get_available_packet_count() > 0:
		_handle_packet(_socket.get_packet())

	_ping_left -= delta
	if _ping_left <= 0.0:
		_send_ping()


func _connect_to_server() -> void:
	_socket = WebSocketPeer.new()
	var err := _socket.connect_to_url(WS_URL)
	if err != OK:
		_state = State.DISCONNECTED
		_retry_left = RETRY_DELAY
		var reason := "No se pudo iniciar la conexión (error %d)" % err
		print("[net] %s — reintentando en %.0f s" % [reason, RETRY_DELAY])
		disconnected.emit(reason)
		return
	_state = State.CONNECTING


func _on_open() -> void:
	_state = State.CONNECTED
	print("[net] conectado")
	connected.emit()
	_send_login()
	_send_ping()


func _send_login() -> void:
	if _user.is_empty():
		return
	_send({"t": "login", "d": {"user": _user}})


func _send_ping() -> void:
	if _send({"t": "ping", "d": {}}):
		_last_ping_msec = Time.get_ticks_msec()
		_ping_left = PING_INTERVAL


func _send(message: Dictionary) -> bool:
	if _state != State.CONNECTED:
		return false
	var err := _socket.send_text(JSON.stringify(message))
	if err != OK:
		push_warning("[net] fallo al enviar mensaje (código %d)" % err)
		return false
	return true


func _handle_packet(packet: PackedByteArray) -> void:
	var text := packet.get_string_from_utf8()
	var parsed: Variant = JSON.parse_string(text)
	if typeof(parsed) != TYPE_DICTIONARY:
		push_warning("[net] mensaje no-JSON: %s" % text)
		return

	var data: Dictionary = parsed
	var mtype := str(data.get("t", ""))
	var payload: Variant = data.get("d", {})
	var d: Dictionary = payload if payload is Dictionary else {}

	match mtype:
		"pong":
			var rtt := Time.get_ticks_msec() - _last_ping_msec
			pong_received.emit(rtt)
		"welcome":
			print("[net] welcome recibido")
			welcome_received.emit(d)
		"snapshot":
			snapshot_received.emit(d)
		"err":
			push_warning("[net] error del servidor: %s" % d)
		_:
			push_warning("[net] mensaje desconocido: %s" % mtype)


func _schedule_retry(reason: String) -> void:
	if _state == State.DISCONNECTED:
		return
	_state = State.DISCONNECTED
	_retry_left = RETRY_DELAY
	print("[net] %s — reintentando en %.0f s" % [reason, RETRY_DELAY])
	_socket.close()
	disconnected.emit(reason)
