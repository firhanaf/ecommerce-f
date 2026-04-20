.PHONY: run build tidy lint

run:
	go run ./cmd/api/main.go

build:
	go build -o bin/api ./cmd/api/main.go

tidy:
	go mod tidy

# jalankan postgres lokal via docker (untuk development)
db:
	docker run --name ecommerce-db \
		-e POSTGRES_USER=postgres \
		-e POSTGRES_PASSWORD=postgres \
		-e POSTGRES_DB=ecommerce_db \
		-p 5432:5432 -d postgres:16-alpine

# apply DDL ke database
migrate:
	psql -U postgres -d ecommerce_db -f migrations/ecommerce_mvp.sql

# stop & hapus container postgres
db-stop:
	docker stop ecommerce-db && docker rm ecommerce-db
