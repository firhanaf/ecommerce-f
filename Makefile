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

# jalankan MinIO (S3-compatible local storage)
# Web UI: http://localhost:9001  |  user: minioadmin  |  pass: minioadmin
minio:
	docker run --name ecommerce-minio \
		-e MINIO_ROOT_USER=minioadmin \
		-e MINIO_ROOT_PASSWORD=minioadmin \
		-p 9000:9000 -p 9001:9001 \
		-v ecommerce-minio-data:/data \
		-d minio/minio server /data --console-address ":9001"

minio-stop:
	docker stop ecommerce-minio && docker rm ecommerce-minio
