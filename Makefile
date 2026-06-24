up:
	docker compose up --build

down:
	docker compose down

logs:
	docker compose logs -f

migrate-new:
	@if [ -z "$(name)" ]; then echo "make migrate-new name=some_name"; exit 1; fi
	@migrate create -ext sql -dir internal/infra/postgres/migrations -seq $(name)


CONFIG_PATH ?=config/config.yaml
run:
	CONFIG_PATH=${CONFIG_PATH} go run cmd/api/main.go