# Emberdeep: common tasks.
#   make dev     Go server + Vite dev server with hot reload, reachable from other computers
#   make test    Go vet + quick Go tests + client tests
#   make build   client bundle + server binary in bin/emberdeep
#   make run     build, then serve everything from the Go server on port 8080
#
# HOST is the address to listen on (0.0.0.0 = every network interface), e.g. `make dev HOST=127.0.0.1`.

HOST     ?= 0.0.0.0
PORT     ?= 8080
WEB_PORT ?= 5173
BOTS     ?= 0
DEPTH    ?= 1

.PHONY: dev test test-all build run web-deps clean

web/node_modules: web/package.json
	cd web && npm install
	@touch web/node_modules

web-deps: web/node_modules

# Both servers run until Ctrl+C, which stops both. Open http://<this computer's IP>:$(WEB_PORT).
# Vite proxies /ws and /api to the Go server, so only port $(WEB_PORT) needs to be reachable.
dev: web/node_modules
	@echo "Emberdeep dev: open http://$$(hostname -I 2>/dev/null | cut -d' ' -f1):$(WEB_PORT) on the other computer"
	@trap 'kill 0' INT TERM EXIT; \
	go run ./cmd/emberdeep -dev -addr :$(PORT) -bots $(BOTS) -depth $(DEPTH) & \
	cd web && npx vite --host $(HOST) --port $(WEB_PORT) --strictPort & \
	wait

test: web/node_modules
	go vet ./...
	go test -short ./...
	cd web && npm test

# Also plays 4 headless bots for 60 s.
test-all: test
	go test ./...

build: web/node_modules
	cd web && npm run build
	go build -o bin/emberdeep ./cmd/emberdeep

run: build
	./bin/emberdeep -addr $(HOST):$(PORT) -bots $(BOTS) -depth $(DEPTH)

clean:
	rm -rf bin web/dist web/.smoke.mjs
