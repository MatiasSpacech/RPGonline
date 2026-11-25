extends Node3D
## F0 — "Hola red": conecta al servidor Go por WebSocket, envía ping
## cada 2 segundos y muestra en pantalla los pong recibidos (con RTT).
## Si el servidor se cae, reintenta solo cada 3 segundos.

enum State { DISCONNECTED, CONNECTING, CONNECTED }

const WS_URL := "ws://127.0.0.1:8080/ws"
const PING_INTERVAL := 2.0
const RETRY_DELAY := 3.0

@onready var _status: Label = $UI/StatusLabel

var _socket := WebSocketPeer.new()
var _state := State.DISCONNECTED
var _retry_left := 0.0
var _ping_left := 0.0
var _last_ping_msec := 0
var _pongs := 0


func _ready() -> void:
	print("[net] iniciando cliente WebSocket -> %s" % WS_URL)
	_connect_to_server()


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
				_on_connected()
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
	_status.text = "Conectando a %s ..." % WS_URL
	var err := _socket.connect_to_url(WS_URL)
	if err != OK:
		_schedule_retry("No se pudo iniciar la conexión (error %d)" % err)
		return
	_state = State.CONNECTING


func _on_connected() -> void:
	_state = State.CONNECTED
	_pongs = 0
	_status.text = "Conectado ✓ — esperando pong..."
	print("[net] conectado")
	_send_ping()


func _send_ping() -> void:
	var msg := JSON.stringify({"t": "ping", "d": {"client": "godot"}})
	var err := _socket.send_text(msg)
	if err != OK:
		_schedule_retry("Error enviando ping (código %d)" % err)
		return
	_last_ping_msec = Time.get_ticks_msec()
	_ping_left = PING_INTERVAL


func _handle_packet(packet: PackedByteArray) -> void:
	var text := packet.get_string_from_utf8()
	var data: Variant = JSON.parse_string(text)
	if typeof(data) != TYPE_DICTIONARY:
		push_warning("[net] mensaje no-JSON: %s" % text)
		return

	var mtype := str(data.get("t", ""))
	if mtype == "pong":
		_pongs += 1
		var rtt := Time.get_ticks_msec() - _last_ping_msec
		_status.text = "Conectado ✓  ·  pong #%d  ·  RTT %d ms" % [_pongs, rtt]
		print("[net] pong #%d rtt=%dms" % [_pongs, rtt])
	else:
		push_warning("[net] mensaje desconocido: %s" % mtype)


func _schedule_retry(reason: String) -> void:
	print("[net] %s — reintentando en %.0f s" % [reason, RETRY_DELAY])
	_state = State.DISCONNECTED
	_retry_left = RETRY_DELAY
	_socket.close()
	_status.text = "%s\nReintentando en %.0f s..." % [reason, RETRY_DELAY]
