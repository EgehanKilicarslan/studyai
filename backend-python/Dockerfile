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

COPY --from=builder /dist /app/libs

ENV PYTHONPATH="/app/libs:/app:$PYTHONPATH"
ENV PATH="/app/libs/bin:$PATH"
ENV HF_HOME="/home/appuser/.cache/huggingface"

COPY app/ ./app/
COPY pb/ ./pb/

RUN chown -R appuser:appgroup /app

EXPOSE 50051

USER appuser

CMD ["python", "app/main.py"]