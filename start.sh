#!/bin/sh
set -e

echo "=== Запуск магазина цветов «Василёк» ==="

docker-entrypoint.sh postgres &
PG_PID=$!

echo "Ожидание PostgreSQL..."
until pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" 2>/dev/null; do
    sleep 1
done
echo "PostgreSQL готов."

echo "Запуск веб-приложения..."
/usr/local/bin/vasilyok /etc/vasilyok/config.ini &
APP_PID=$!

wait -n "$PG_PID" "$APP_PID"
