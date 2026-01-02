#!/bin/sh
set -e

# Change to app directory
cd /app/app

# Run migrations only if not a celery worker
if [ "$1" != "celery" ]; then
    echo "Running Alembic migrations..."
    cd /app && alembic upgrade head
    cd /app/app
fi

# If a command was passed, execute it
if [ $# -gt 0 ]; then
    echo "Starting: $@"
    exec "$@"
else
    # Default: start the gRPC server
    echo "Starting gRPC server..."
    exec python /app/app/main.py
fi