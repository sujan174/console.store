# console.store — local live dev runbook.
#
# One-time:   make db-up
# Two shells: make broker     (shell A — holds Swiggy tokens, talks to Swiggy)
#             make sshd       (shell B — the SSH-facing TUI)
# Then:       make ssh        (shell C — connect, authorize once, order)
#
# Real orders are OFF by default. See `make live-orders` before placing one.

SHELL := /bin/bash
COMPOSE := docker compose
PSQL := $(COMPOSE) exec -T postgres psql -U console_owner -d console

.DEFAULT_GOAL := help

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'

## db-up: start Postgres (Docker) and apply schema on first init
db-up:
	$(COMPOSE) up -d
	@echo "waiting for postgres to be healthy..."
	@until [ "$$($(COMPOSE) ps -q postgres | xargs docker inspect -f '{{.State.Health.Status}}' 2>/dev/null)" = "healthy" ]; do sleep 1; done
	@echo "postgres healthy on localhost:5432"

## db-migrate: (re)apply schema.sql idempotently as the owner role
db-migrate:
	$(PSQL) < internal/store/schema.sql
	@echo "schema applied"

## db-psql: open a psql shell as the owner
db-psql:
	$(COMPOSE) exec postgres psql -U console_owner -d console

## db-reset: DESTROY the db volume and recreate (forces schema re-init)
db-reset:
	$(COMPOSE) down -v
	$(MAKE) db-up

## db-down: stop Postgres (keeps the data volume)
db-down:
	$(COMPOSE) down

## keygen: write a fresh local-KMS master key into .env.local if missing
keygen:
	@if [ -f .env.local ] && grep -q '^CONSOLE_KMS_MASTER_KEY=' .env.local; then \
		echo ".env.local already has a master key — leaving it alone"; \
	else \
		echo "CONSOLE_KMS_MASTER_KEY=$$(head -c 32 /dev/urandom | base64)" >> .env.local; \
		echo "appended a fresh master key to .env.local"; \
	fi

## broker: run the privileged broker (loads .env.local). Keep this shell open.
broker:
	@set -a; . ./.env.local; set +a; go run ./cmd/broker

## sshd: run the SSH-facing TUI in live mode (loads .env.local). Keep open.
sshd:
	@set -a; . ./.env.local; set +a; CONSOLE_BACKEND=live go run ./cmd/sshd

## ssh: connect to the local TUI
ssh:
	ssh localhost -p 2222

## live-orders: print how to arm real COD order placement
live-orders:
	@echo "Real, NON-CANCELLABLE COD orders are gated by CONSOLE_LIVE_ORDERS=1."
	@echo "To arm: uncomment CONSOLE_LIVE_ORDERS=1 in .env.local, then restart 'make broker'."
	@echo "Leave it unset to browse and build carts safely (place_order is refused)."

.PHONY: help db-up db-migrate db-psql db-reset db-down keygen broker sshd ssh live-orders
