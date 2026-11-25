# MuNext — atajos de desarrollo (F0)
#
# Requiere que `go` y `godot` estén en el PATH
# (ver README: se instalan en ~/.local y ~/.bashrc agrega esa ruta).

GO ?= go
GODOT ?= godot

.PHONY: server test fmt vet client build-server

server: ## Arranca el servidor en ws://127.0.0.1:8080/ws
	cd server && $(GO) run ./cmd/munext

test: ## Tests unitarios + integración del servidor
	cd server && $(GO) test ./...

fmt: ## Formatea el código Go
	cd server && $(GO) fmt ./...

vet: ## Análisis estático de Go
	cd server && $(GO) vet ./...

client: ## Abre/ejecuta el cliente Godot
	$(GODOT) --path client

build-server: ## Compila el binario del servidor en server/bin/
	cd server && $(GO) build -o bin/munext ./cmd/munext
