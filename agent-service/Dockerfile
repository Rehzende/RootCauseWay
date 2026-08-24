FROM python:3.12-slim AS builder
WORKDIR /app
RUN pip install --no-cache-dir --upgrade pip
COPY pyproject.toml .
RUN pip install --no-cache-dir --prefix=/install .

FROM python:3.12-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends curl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r rootcauseway && useradd -r -g rootcauseway rootcauseway
WORKDIR /app
COPY --from=builder /install /usr/local
COPY . .
RUN chown -R rootcauseway:rootcauseway /app
USER rootcauseway
EXPOSE 8081
HEALTHCHECK --interval=15s --timeout=5s --retries=3 --start-period=15s \
    CMD curl -f http://localhost:8081/health || exit 1
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8081"]
