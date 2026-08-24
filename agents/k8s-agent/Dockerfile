FROM python:3.12-slim AS builder
WORKDIR /app
RUN pip install --no-cache-dir --upgrade pip
COPY pyproject.toml .
RUN pip install --no-cache-dir --prefix=/install .

FROM python:3.12-slim AS runtime
# Install kubectl for in-cluster use
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates \
    && KUBECTL_VER=$(curl -Ls https://dl.k8s.io/release/stable.txt) \
    && curl -LO "https://dl.k8s.io/release/${KUBECTL_VER}/bin/linux/amd64/kubectl" \
    && install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl \
    && rm kubectl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r rootcauseway && useradd -r -g rootcauseway rootcauseway

WORKDIR /app
COPY --from=builder /install /usr/local
COPY app/ app/

RUN chown -R rootcauseway:rootcauseway /app
USER rootcauseway
EXPOSE 8094
HEALTHCHECK --interval=30s --timeout=5s --retries=3 --start-period=10s \
    CMD curl -f http://localhost:8094/health || exit 1
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8094"]
