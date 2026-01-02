# ==========================================
# 🏗️ STAGE 1: BUILDER
# ==========================================
FROM python:3.11-slim AS builder

COPY --from=ghcr.io/astral-sh/uv:latest /uv /bin/uv

WORKDIR /build

COPY pyproject.toml uv.lock ./

RUN uv pip install --system --target=/dist --no-cache -r pyproject.toml

# ==========================================
# 🚀 STAGE 2: RUNNER
# ==========================================
FROM python:3.11-slim

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

WORKDIR /app

RUN addgroup --system appgroup && adduser --system --group appuser

RUN mkdir -p /home/appuser/.cache/huggingface && \
    mkdir -p /home/appuser/.cache/models && \
    chown -R appuser:appgroup /home/appuser/.cache

RUN mkdir -p /tmp/studyai/uploads && \
    chown -R appuser:appgroup /tmp/studyai/uploads

COPY --from=builder /dist /app/libs

ENV PYTHONPATH="/app/libs:/app:$PYTHONPATH"
ENV PATH="/app/libs/bin:$PATH"
ENV HF_HOME="/home/appuser/.cache/huggingface"

COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh

COPY app/ ./app/
COPY pb/ ./pb/
COPY alembic.ini alembic.ini

RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 50051

ENTRYPOINT ["/app/docker-entrypoint.sh"]