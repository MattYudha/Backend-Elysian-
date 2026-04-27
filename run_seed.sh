#!/bin/bash
echo "Menjalankan seeder admin..."
sudo docker run --rm --network host -v "$PWD:/app" -w /app golang:alpine sh -c "go mod download && go run cmd/seed_admin/main.go"
echo "Selesai!"
