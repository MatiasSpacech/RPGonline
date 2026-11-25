# MuNext — atajos de desarrollo (F0)
#
# Requiere que `go` y `godot` estén en el PATH
# (ver README: se instalan en ~/.local y ~/.bashrc agrega esa ruta).

GO ?= go
GODOT ?= godot

.PHONY: server test test-db fmt vet client client-autotest build-server devdb

server: ## Arranca el servidor en ws://127.0.0.1:8080/ws (requiere DB)
	cd server && $(GO) run ./cmd/munext

devdb: ## Postgres embebido para desarrollo (sin Docker), puerto 5432
	cd server && $(GO) run ./cmd/devdb

test: ## Tests unitarios + integración en memoria (sin base de datos)
	cd server && $(GO) test ./...

test-db: ## Tests completos incluyendo Postgres (requiere `make devdb`)
	cd server && MUNEXT_TEST_DATABASE_URL="$${MUNEXT_TEST_DATABASE_URL:-postgres://postgres:postgres@127.0.0.1:5432/munext?sslmode=disable}" $(GO) test -p 1 ./... -count=1

fmt: ## Formatea el código Go
	cd server && $(GO) fmt ./...

vet: ## Análisis estático de Go
	cd server && $(GO) vet ./...

client: ## Abre/ejecuta el cliente Godot
	$(GODOT) --path client

client-autotest: ## Smoke test headless: registro/login → personaje → mundo
	$(GODOT) --headless --path client -- --autotest

build-server: ## Compila el binario del servidor en server/bin/
	cd server && $(GO) build -o bin/munext ./cmd/munext
