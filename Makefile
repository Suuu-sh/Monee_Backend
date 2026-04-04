.PHONY: deps test docker-up docker-down docker-logs

deps:
	docker run --rm -v $(PWD):/app -w /app golang:1.24-bookworm sh -c "go mod tidy"

test:
	docker run --rm -v $(PWD):/app -w /app golang:1.24-bookworm sh -c "go test ./..."

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f api
