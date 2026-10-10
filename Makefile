# Atish
#
# Run `make help` for the command list.
#
# Requires: docker (Docker Desktop), go 1.25+, node 20+.
# Works from PowerShell, cmd, or Git Bash — see the shell block below.

# ------------------------------------------------------------------- shell
# The recipes are bash (pipes, grep, [[ ]], $(...)). On Windows, make defaults
# to cmd.exe and a bare `SHELL := /bin/bash` does not resolve, so point SHELL
# at Git Bash by absolute path. The 8.3 short path (PROGRA~1) is deliberate:
# GNU Make 3.81 (GnuWin32) mis-parses a SHELL containing spaces.
# This block must come before any $(shell ...) call, which also uses SHELL.

ifeq ($(OS),Windows_NT)
    GIT_BASH_CANDIDATES := \
        C:/PROGRA~1/Git/bin/bash.exe \
        C:/PROGRA~2/Git/bin/bash.exe \
        $(subst \,/,$(LOCALAPPDATA))/Programs/Git/bin/bash.exe

    GIT_BASH := $(firstword $(foreach b,$(GIT_BASH_CANDIDATES),$(wildcard $(b))))

    ifeq ($(GIT_BASH),)
        $(error Git Bash not found. Install Git for Windows, or run make from a \
            Git Bash prompt. Looked in: $(GIT_BASH_CANDIDATES))
    endif

    SHELL := $(GIT_BASH)
else
    SHELL := /bin/bash
endif

.DEFAULT_GOAL := help

# ---------------------------------------------------------------- settings
# Every value can be overridden on the command line:  make run API_PORT=9000
# Production secrets live in .env (see `make env`); this file never reads them.

BACKEND_DIR   ?= backend
FRONTEND_DIR  ?= frontend

API_PORT      ?= 8080
WEB_PORT      ?= 8080
API_URL       ?= http://127.0.0.1:$(WEB_PORT)

# Development infrastructure (docker-compose.dev.yml). Non-default ports so a
# native PostgreSQL/Redis never collides.
DEV_DB_PORT    ?= 55432
DEV_REDIS_PORT ?= 56379
DEV_PASSWORD   ?= supersecret-admin-pw

# Docker Desktop on Windows does not put docker on PATH for the non-login shell
# make spawns. Resolve it here with $(wildcard) and export the result so the
# scripts do not have to repeat the search.
DOCKER ?= docker

ifeq ($(OS),Windows_NT)
    DOCKER_CHECK := $(shell docker version 2>/dev/null)
    ifeq ($(DOCKER_CHECK),)
        DOCKER_CANDIDATES := \
            C:/PROGRA~1/Docker/Docker/resources/bin/docker.exe \
            C:/PROGRA~2/Docker/Docker/resources/bin/docker.exe \
            $(subst \,/,$(LOCALAPPDATA))/Programs/Docker/Docker/resources/bin/docker.exe \
            $(subst \,/,$(LOCALAPPDATA))/Programs/DockerDesktop/resources/bin/docker.exe

        DOCKER := $(firstword $(foreach d,$(DOCKER_CANDIDATES),$(wildcard $(d))))
        ifeq ($(DOCKER),)
            DOCKER := docker
        endif
    endif
endif

COMPOSE     = $(DOCKER) compose
DEV_COMPOSE = $(DOCKER) compose -f docker-compose.dev.yml

# NOTE: deliberately not a bare `export`. On Windows a bare export sends every
# unset make variable to recipes as an empty string, which blanks real
# environment variables (LOCALAPPDATA, USERNAME…). Export only what the
# scripts need.
export DOCKER DEV_DB_PORT DEV_REDIS_PORT

# Development environment for `go run` (the API and the admin CLI).
DEV_ENV = DATABASE_URL="postgres://atish:atish@127.0.0.1:$(DEV_DB_PORT)/atish?sslmode=disable" \
          REDIS_URL="redis://127.0.0.1:$(DEV_REDIS_PORT)/0" \
          DEV_AUTH=true ADMIN_USERNAME=admin ADMIN_PASSWORD="$(DEV_PASSWORD)" \
          MEDIA_DIR=./data/media PORT=$(API_PORT) MINIAPP_URL=http://localhost:5173 \
          PUBLIC_URL=http://localhost:$(API_PORT)

# All admin-* targets go through scripts/admin.sh, which talks to the running
# production container if there is one, else the local dev database.
# Force with PROD=1 / PROD=0.
ADMIN = PROD="$(PROD)" bash scripts/admin.sh

.PHONY: help env secrets \
        up up-tls up-prebuilt down restart logs ps clean update \
        images images-bundle load-images bootstrap ssl \
        dev-up dev-down dev-reset run run-frontend demo stop stop-all \
        build build-backend build-frontend \
        db-shell db-dump db-restore backup \
        admin-stats admin-users admin-user admin-promote admin-demote \
        admin-ban admin-suspend admin-activate admin-verify \
        admin-premium admin-unpremium admin-delete-user admin-plans \
        admin-settings admin-set admin-maintenance admin-banner \
        admin-registration admin-premium-mode admin-reports admin-resolve \
        admin-audit admin-broadcast admin-password admin-url \
        admin-set-password admin-clear-password \
        health smoke test test-all lint fmt tidy deploy-check deploy-health

# -------------------------------------------------------------------- help
help: ## Show this help
	@echo ""
	@echo "Atish - available targets"
	@echo ""
	@# Colour only when stdout is a terminal, so piping the output stays clean.
	@if [ -t 1 ]; then C=$$'\033[36m'; R=$$'\033[0m'; else C=""; R=""; fi; \
	grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| sort \
		| awk -v c="$$C" -v r="$$R" 'BEGIN {FS = ":.*?## "}; {printf "  %s%-22s%s %s\n", c, $$1, r, $$2}'
	@echo ""
	@echo "Develop:   make dev-up && make run   (then, in another terminal: make run-frontend, make demo)"
	@echo "Deploy:    make env && make up       (or on a fresh VPS: ./scripts/bootstrap-vps.sh)"
	@echo "Operate:   make admin-users | admin-user U=... | admin-ban U=... REASON=... | admin-premium U=... DAYS=30"
	@echo ""

# ------------------------------------------------------------ configuration
env: ## Create .env with freshly generated secrets (never overwrites)
	@bash scripts/init-env.sh

secrets: ## Print fresh JWT_SECRET / ENCRYPTION_KEY values
	@cd $(BACKEND_DIR) && go run ./cmd/cli keygen

# ---------------------------------------------------- docker stack (prod-like)
up: ## Build and start the full stack (Postgres, Redis, API, web)
	@test -f .env || { echo "No .env yet. Run: make env"; exit 1; }
	$(COMPOSE) up -d --build
	@echo "Waiting for the API to become healthy..."
	@for i in $$(seq 1 90); do \
		st=$$($(COMPOSE) ps backend --format '{{.Health}}' 2>/dev/null | head -1); \
		if [ "$$st" = "healthy" ]; then echo "API healthy."; exit 0; fi; \
		sleep 1; \
	done; \
	echo "API did not become healthy in 90s. Check: make logs SERVICE=backend"; exit 1

up-tls: ## Same as up, plus the built-in Caddy reverse proxy with automatic HTTPS (set DOMAIN in .env)
	@test -f .env || { echo "No .env yet. Run: make env"; exit 1; }
	$(COMPOSE) --profile tls up -d --build

up-prebuilt: ## Start from images loaded with make load-images, without building
	@test -f .env || { echo "No .env yet. Run: make env"; exit 1; }
	$(COMPOSE) up -d --no-build

down: ## Stop the stack (data is kept)
	$(COMPOSE) down

restart: down up ## Restart the stack

logs: ## Tail logs (SERVICE=backend to narrow)
	$(COMPOSE) logs -f --tail=100 $(SERVICE)

ps: ## Container status
	$(COMPOSE) ps

clean: ## Stop the stack AND delete its volumes (destroys all data)
	@read -r -p "This deletes the database and every uploaded photo. Type 'yes' to continue: " a; \
	[ "$$a" = "yes" ] || { echo "Cancelled."; exit 1; }
	$(COMPOSE) down -v

update: ## Pull the latest code and rebuild (git pull + up)
	git pull --ff-only
	$(MAKE) up

images: ## Build both production images without starting anything
	$(DOCKER) build -t atish-backend:$(or $(VERSION),latest) ./$(BACKEND_DIR)
	$(DOCKER) build -t atish-frontend:$(or $(VERSION),latest) ./$(FRONTEND_DIR)
	@$(DOCKER) images --format "{{.Repository}}:{{.Tag}}  {{.Size}}" | grep atish

images-bundle: ## Build the images on THIS machine and pack them for a low-resource VPS
	@bash scripts/build-images.sh

load-images: ## Load an images-bundle tarball (run this ON the server). FILE=path optional
	@bash scripts/load-images.sh $(FILE)

bootstrap: ## One-shot setup on a fresh Ubuntu/Debian VPS: Docker, .env, stack, nginx + TLS. DOMAIN= LETSENCRYPT_EMAIL=
	@bash scripts/bootstrap-vps.sh

ssl: ## Configure host nginx + Let's Encrypt in front of the web container (CHALLENGE=dns for DNS-01)
	@bash scripts/deploy-host-nginx.sh

# ------------------------------------------------------------- development
dev-up: ## Start dev Postgres + Redis (ports 55432 and 56379)
	$(DEV_COMPOSE) up -d --wait

dev-down: ## Stop the dev infrastructure (data is kept)
	$(DEV_COMPOSE) down

dev-reset: ## Wipe the dev database and Redis (the API re-creates the schema on next start)
	$(DEV_COMPOSE) down -v
	$(DEV_COMPOSE) up -d --wait
	@rm -rf data
	@echo "Dev data wiped. Start the API again with: make run"

run: ## Run the API with DEV_AUTH on (Ctrl+C to stop)
	@cd $(BACKEND_DIR) && $(DEV_ENV) go run ./cmd/server

run-frontend: ## Run the Vite dev server on :5173, proxying /api to the API
	@cd $(FRONTEND_DIR) && npm run dev

demo: ## Load 10 demo profiles into the running dev API
	@bash scripts/demo-data.sh

stop: ## Kill whatever listens on the API and Vite ports (for servers left in the background)
	@for port in $(API_PORT) 5173; do \
		pid=$$(netstat -ano 2>/dev/null | grep -E "LISTENING" | grep -E ":$$port[[:space:]]" | awk '{print $$NF}' | head -1); \
		if [ -n "$$pid" ]; then \
			echo "Stopping PID $$pid on port $$port"; \
			taskkill //F //PID $$pid >/dev/null 2>&1 || kill -9 $$pid 2>/dev/null || true; \
		else \
			echo "Nothing listening on port $$port"; \
		fi; \
	done

stop-all: stop dev-down ## Stop the dev servers and the dev containers

build: build-backend build-frontend ## Build backend binaries and the frontend bundle

build-backend: ## Compile server + cli into backend/bin
	@cd $(BACKEND_DIR) && go build -o bin/server ./cmd/server && go build -o bin/cli ./cmd/cli
	@echo "Binaries in $(BACKEND_DIR)/bin/"

build-frontend: ## Build the production frontend bundle
	@cd $(FRONTEND_DIR) && npm ci --no-audit --no-fund && npm run build

# ---------------------------------------------------------------- database
db-shell: ## Open psql (production stack if running, else dev DB)
	@if $(COMPOSE) ps -q postgres 2>/dev/null | grep -q .; then \
		$(COMPOSE) exec postgres psql -U atish -d atish; \
	else \
		$(DEV_COMPOSE) exec postgres psql -U atish -d atish; \
	fi

db-dump: ## Dump the database to FILE (default backup.sql)
	@$(COMPOSE) exec -T postgres pg_dump -U atish -d atish --clean --if-exists > $(or $(FILE),backup.sql)
	@echo "Wrote $(or $(FILE),backup.sql)"

db-restore: ## Restore from FILE (default backup.sql) into the production stack
	@test -f $(or $(FILE),backup.sql) || { echo "$(or $(FILE),backup.sql) not found"; exit 1; }
	@$(COMPOSE) exec -T postgres psql -U atish -d atish -v ON_ERROR_STOP=1 < $(or $(FILE),backup.sql)
	@echo "Restored from $(or $(FILE),backup.sql)"

backup: ## Timestamped backup: database dump + photos archive into backups/
	@mkdir -p backups
	@ts=$$(date +%Y%m%d-%H%M%S); \
	$(COMPOSE) exec -T postgres pg_dump -U atish -d atish --clean --if-exists | gzip > backups/atish-$$ts.sql.gz; \
	$(COMPOSE) exec -T backend tar -C /data -czf - media > backups/atish-media-$$ts.tar.gz; \
	echo "Wrote backups/atish-$$ts.sql.gz and backups/atish-media-$$ts.tar.gz"

# ------------------------------------------------------------------- admin
# Operator tools. U (or any REF) can be: Atish username, name, Telegram id,
# @telegram_username, or the user's UUID. Add PROD=1 to force the running
# containers; by default they are used automatically when the stack is up.

admin-stats: ## Platform counters (users, matches, messages, reports, revenue)
	@$(ADMIN) stats

admin-users: ## List users. [Q=text] [STATUS=active|suspended|banned] [ROLE=] [PREMIUM=yes|no] [LIMIT=25] [PAGE=1]
	@$(ADMIN) list-users -q "$(Q)" -status "$(STATUS)" -role "$(ROLE)" -premium "$(PREMIUM)" -limit $(or $(LIMIT),25) -page $(or $(PAGE),1)

admin-user: ## Show one user in full. U=...
	@test -n "$(U)" || { echo "U is required: make admin-user U=Atish_alex"; exit 1; }
	@$(ADMIN) user -user "$(U)"

admin-promote: ## Make a Telegram user an admin. TG=<telegram id> [ROLE=admin|moderator]
	@test -n "$(TG)" || { echo "TG is required: make admin-promote TG=123456789"; exit 1; }
	@$(ADMIN) promote -telegram-id "$(TG)" -role "$(or $(ROLE),admin)"

admin-demote: ## Remove a user's staff role. U=...
	@test -n "$(U)" || { echo "U is required"; exit 1; }
	@$(ADMIN) demote -user "$(U)"

admin-ban: ## Ban a user (cut off within seconds). U=... [REASON="..."]
	@test -n "$(U)" || { echo "U is required: make admin-ban U=Atish_alex REASON=spam"; exit 1; }
	@$(ADMIN) set-status -user "$(U)" -status banned -reason "$(REASON)"

admin-suspend: ## Temporarily suspend a user. U=... [REASON="..."]
	@test -n "$(U)" || { echo "U is required"; exit 1; }
	@$(ADMIN) set-status -user "$(U)" -status suspended -reason "$(REASON)"

admin-activate: ## Lift a ban or suspension. U=...
	@test -n "$(U)" || { echo "U is required"; exit 1; }
	@$(ADMIN) set-status -user "$(U)" -status active

admin-verify: ## Set a verification badge. U=... FLAG=telegram|phone|photo|identity [VALUE=true|false]
	@test -n "$(U)" -a -n "$(FLAG)" || { echo "U and FLAG are required: make admin-verify U=Atish_alex FLAG=photo"; exit 1; }
	@$(ADMIN) verify -user "$(U)" -flag "$(FLAG)" -value=$(or $(VALUE),true)

admin-premium: ## Grant Atish Plus. U=... [PLAN=premium_month] [DAYS=30] (DAYS=0 is lifetime)
	@test -n "$(U)" || { echo "U is required: make admin-premium U=Atish_alex DAYS=30"; exit 1; }
	@$(ADMIN) grant-premium -user "$(U)" -plan "$(or $(PLAN),premium_month)" -days $(or $(DAYS),30)

admin-unpremium: ## Revoke Atish Plus. U=...
	@test -n "$(U)" || { echo "U is required"; exit 1; }
	@$(ADMIN) revoke-premium -user "$(U)"

admin-delete-user: ## Permanently erase an account. U=...
	@test -n "$(U)" || { echo "U is required"; exit 1; }
	@read -r -p "Erase $(U) and all their data forever? Type 'yes': " a; [ "$$a" = "yes" ] || { echo "Cancelled."; exit 1; }
	@$(ADMIN) delete-user -user "$(U)" -yes

admin-plans: ## List premium plans
	@$(ADMIN) plans

admin-settings: ## Show every platform setting
	@$(ADMIN) settings

admin-set: ## Change a setting. KEY=free_daily_likes VALUE=100
	@test -n "$(KEY)" || { echo "KEY and VALUE are required: make admin-set KEY=free_daily_likes VALUE=100"; exit 1; }
	@$(ADMIN) set-setting -key "$(KEY)" -value '$(VALUE)'

admin-maintenance: ## Maintenance mode on/off. ON=1 or ON=0
	@test -n "$(ON)" || { echo "ON is required: make admin-maintenance ON=1"; exit 1; }
	@$(ADMIN) set-setting -key maintenance_mode -value $$([ "$(ON)" = "1" ] && echo true || echo false)

admin-banner: ## Set the in-app announcement banner (empty text clears it). TEXT="..."
	@$(ADMIN) set-setting -key banner -value '"$(TEXT)"'

admin-registration: ## Open or close new sign-ups. OPEN=1 or OPEN=0
	@test -n "$(OPEN)" || { echo "OPEN is required: make admin-registration OPEN=0"; exit 1; }
	@$(ADMIN) set-setting -key registration_open -value $$([ "$(OPEN)" = "1" ] && echo true || echo false)

admin-premium-mode: ## Premium on (paid features) or off (everything free). ON=1 or ON=0
	@test -n "$(ON)" || { echo "ON is required: make admin-premium-mode ON=0"; exit 1; }
	@$(ADMIN) set-setting -key premium_enabled -value $$([ "$(ON)" = "1" ] && echo true || echo false)

admin-reports: ## Moderation queue. [STATUS=open|reviewing|resolved|dismissed]
	@$(ADMIN) reports -status "$(or $(STATUS),open)"

admin-resolve: ## Close a report. ID=... [STATUS=resolved|dismissed] [ACTION=suspend|ban] [REASON="..."]
	@test -n "$(ID)" || { echo "ID is required: make admin-resolve ID=3 ACTION=ban REASON=scam"; exit 1; }
	@$(ADMIN) resolve-report -id $(ID) -status "$(or $(STATUS),resolved)" -action "$(ACTION)" -resolution "$(REASON)"

admin-audit: ## Recent admin actions. [LIMIT=30]
	@$(ADMIN) audit -limit $(or $(LIMIT),30)

admin-broadcast: ## Message every active user through the bot. TEXT="..."
	@test -n "$(TEXT)" || { echo "TEXT is required: make admin-broadcast TEXT=\"We just shipped chat photos!\""; exit 1; }
	@read -r -p "Send this to ALL users? Type 'yes': " a; [ "$$a" = "yes" ] || { echo "Cancelled."; exit 1; }
	@$(ADMIN) broadcast -text "$(TEXT)" -yes

admin-set-password: ## Set an admin-panel password for an admin/moderator (promoted ones too). U=... [PASSWORD=...]
	@test -n "$(U)" || { echo "U is required: make admin-set-password U=@HeIsAbolfazl"; exit 1; }
	@$(ADMIN) set-password -user "$(U)" $(if $(PASSWORD),-password "$(PASSWORD)")

admin-clear-password: ## Remove an admin's panel password (Telegram sign-in keeps working). U=...
	@test -n "$(U)" || { echo "U is required"; exit 1; }
	@$(ADMIN) clear-password -user "$(U)"

admin-password: ## Rotate the shared root password (ADMIN_PASSWORD in .env) and restart the API. [PASSWORD=...]
	@PASSWORD="$(PASSWORD)" bash scripts/rotate-admin-password.sh

admin-url: ## Print the admin panel address
	@bash -c 'set -a; [ -f .env ] && . ./.env; set +a; echo "$${PUBLIC_URL:-http://localhost:$(WEB_PORT)}/admin"'

# ----------------------------------------------------------------- quality
health: ## Check every service of the running stack
	@bash scripts/health.sh

smoke: ## End-to-end API test against a DEV_AUTH API (BASE=... to point elsewhere)
	@bash scripts/smoke-test.sh

test: ## Go unit tests and the frontend typecheck
	@echo "--- Go tests ---"
	@cd $(BACKEND_DIR) && go test ./... 2>&1 | grep -v "no test files" || true
	@echo "--- Frontend typecheck ---"
	@cd $(FRONTEND_DIR) && npx tsc --noEmit && echo "OK"

test-all: test smoke ## Unit tests, typecheck, then the end-to-end smoke test

lint: ## Vet the Go code
	@cd $(BACKEND_DIR) && go vet ./...

fmt: ## Format Go code
	@cd $(BACKEND_DIR) && gofmt -w ./cmd ./internal

tidy: ## Tidy Go module dependencies
	@cd $(BACKEND_DIR) && go mod tidy

deploy-check: ## Pre-flight checks before deploying (secrets, build, repo hygiene)
	@bash scripts/deploy-check.sh

deploy-health: ## Check a deployed instance. HOST=https://your.domain
	@test -n "$(HOST)" || { echo "HOST is required: make deploy-health HOST=https://atish.example.com"; exit 1; }
	@API_URL="$(HOST)" REMOTE=1 bash scripts/health.sh
