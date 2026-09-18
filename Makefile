.PHONY: start stop status init serve-api web-dev web-install

start:
	chmod +x scripts/lifecycle.sh
	./scripts/lifecycle.sh start

stop:
	chmod +x scripts/lifecycle.sh
	./scripts/lifecycle.sh stop

status:
	chmod +x scripts/lifecycle.sh
	./scripts/lifecycle.sh status

init: web-install
	@echo "ready: make start / make stop"

web-install:
	cd web && npm install

serve-api:
	go run ./cmd/shopmind serve --port 8788

web-dev:
	cd web && npm run dev
