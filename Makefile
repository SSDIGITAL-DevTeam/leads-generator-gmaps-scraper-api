COMPOSE_FILE := deployments/docker/docker-compose.yml

.PHONY: up down migrate seed api worker

up:
	@docker compose -f $(COMPOSE_FILE) up -d

down:
	@docker compose -f $(COMPOSE_FILE) down

migrate:
	@bash ./scripts/migrate.sh

seed:
	@bash ./scripts/seed.sh

api:
	@cd api && go run ./cmd/api

worker:
	@cd worker && python -m src.jobs.run_query_server
