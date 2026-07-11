#!/bin/bash
echo "Menjalankan seeder admin..."
sudo docker run --rm --network host -e DB_HOST=127.0.0.1 -v "$PWD:/app" -w /app golang:alpine sh -c "go mod download && go run cmd/seed_admin/main.go"
echo "Selesai!"
