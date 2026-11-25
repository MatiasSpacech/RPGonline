class_name NetClient
extends Node
## Cliente WebSocket F2: autenticación, lobby de personajes, token de sesión
## (resume), snapshots y reconexión automática.

signal connected
signal disconnected(reason: String)
signal login_ok(data: Dictionary)
signal char_created(data: Dictionary)
signal char_deleted(char_id: int)
signal welcome_received(data: Dictionary)
signal snapshot_received(data: Dictionary)
signal pong_received(rtt_ms: int)
signal server_error(code: String, message: String)
signal self_state_received(data: Dictionary)
signal damage_received(data: Dictionary)
signal mob_dead_received(data: Dictionary)
signal death_received(data: Dictionary)
signal levelup_received(data: Dictionary)
signal teleport_received(data: Dictionary)

enum State { DISCONNECTED, CONNECTING, CONNECTED }

const WS_URL := "ws://127.0.0.1:8080/ws"
const PING_INTERVAL := 2.0
const RETRY_DELAY := 3.0

var _socket := WebSocketPeer.new()
var _state := State.DISCONNECTED
var _retry_left := 0.0
var _ping_left := 0.0
var _last_ping_msec := 0
var _token := ""
var _pending: Dictionary = {}


func _ready() -> void:
	_connect_to_server()


func request_login(user: String, password: String) -> void:
	_queue_or_send({"t": "login", "d": {"user": user, "pass": password}})


func request_register(user: String, password: String) -> void:
	_queue_or_send({"t": "register", "d": {"user": user, "pass": password}})


func create_character(character_name: String, class_id: String) -> void:
	_send({"t": "create_char", "d": {"name": character_name, "class": class_id}})


func delete_character(character_id: int) -> void:
	_send({"t": "delete_char", "d": {"char_id": character_id}})


func select_character(character_id: int) -> void:
	_send({"t": "select_char", "d": {"char_id": character_id}})


func send_move(point: Vector3) -> void:
	if _state != State.CONNECTED:
		return
	_send({"t": "move_req", "d": {"x": point.x, "y": 0.0, "z": point.z}})


func attack(target_id: int) -> void:
	_send({"t": "attack", "d": {"target": target_id}})


func use_potion(kind: String) -> void:
	_send({"t": "potion", "d": {"kind": kind}})


func alloc_stat(stat: String) -> void:
	_send({"t": "alloc_stat", "d": {"stat": stat}})


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

	if not _token.is_empty():
		# Reconexión: reanuda la sesión sin volver a pedir contraseña.
		_send({"t": "resume", "d": {"token": _token}})
	elif not _pending.is_empty():
		_send(_pending)
		_pending = {}

	_send_ping()


func _send_ping() -> void:
	if _send({"t": "ping", "d": {}}):
		_last_ping_msec = Time.get_ticks_msec()
		_ping_left = PING_INTERVAL


func _queue_or_send(message: Dictionary) -> void:
	if _state == State.CONNECTED:
		_send(message)
	else:
		_pending = message


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
		"login_ok":
			_token = str(d.get("token", ""))
			login_ok.emit(d)
		"char_created":
			char_created.emit(d)
		"char_deleted":
			char_deleted.emit(int(d.get("char_id", 0)))
		"welcome":
			print("[net] welcome recibido")
			welcome_received.emit(d)
		"snapshot":
			snapshot_received.emit(d)
		"self_state":
			self_state_received.emit(d)
		"dmg":
			damage_received.emit(d)
		"mob_dead":
			mob_dead_received.emit(d)
		"death":
			death_received.emit(d)
		"levelup":
			levelup_received.emit(d)
		"teleport":
			teleport_received.emit(d)
		"err":
			var code := str(d.get("code", "error"))
			var message := str(d.get("msg", "Error del servidor"))
			if code == "session_expired":
				_token = ""
			print("[net] error del servidor: %s (%s)" % [message, code])
			server_error.emit(code, message)
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
