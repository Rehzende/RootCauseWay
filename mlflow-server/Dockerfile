# MLflow Tracking Server for RootCauseway's GenAI/LLM tracing.
#
# Backend store: Postgres (the "mlflow" database on rootcauseway-postgres).
# Artifact store: local filesystem, backed by a PVC (mlflow tracing spans
# themselves live in the backend store; the artifact root is only used for
# larger blobs, so a small PVC is enough for this homelab deployment).
FROM python:3.12-slim

# Pinned to match the mlflow-skinny version the 6 traced RootCauseway services
# actually resolve at their own build time (all pin "mlflow-skinny>=2.19.0",
# an open lower bound -- found live running 3.15.1 already). A 3.x client
# writing traces against a 2.19.0 server is the confirmed root cause of a
# live UI bug ("Cannot read properties of undefined (reading 'span_id')")
# viewing any trace recorded after the client side had already moved to 3.x.
RUN pip install --no-cache-dir \
    "mlflow==3.15.1" \
    "psycopg2-binary>=2.9.10" \
    "gunicorn>=23.0.0"

RUN useradd -m -u 1000 mlflow
RUN mkdir -p /mlflow-artifacts && chown mlflow:mlflow /mlflow-artifacts
USER mlflow

EXPOSE 5000

# --workers 2: mlflow's default gunicorn worker count (cpu_count*2+1) OOM'd
# a 768Mi pod almost immediately -- each sync worker imports mlflow's full
# server package (pandas/sklearn/matplotlib included even though unused by
# tracing alone). 2 workers is plenty for this homelab's traffic.
#
# --default-artifact-root mlflow-artifacts:/ (not a bare /mlflow-artifacts
# path): with --serve-artifacts, this special scheme tells every remote
# client (the 6 traced RootCauseway services, each in its own pod/filesystem) to
# proxy artifact reads/writes through this server's HTTP API. A bare
# filesystem path here makes clients assume *they* have direct access to
# that path on *their own* local disk -- which they don't -- and traces
# fail with "Permission denied: '/mlflow-artifacts'" (seen from
# agent-service's pod, not this one) since spans are stored as artifacts.
#
# --allowed-hosts: MLflow 3.5+ added DNS-rebinding protection (a Host
# header allowlist) that isn't part of the "localhost and private IPs"
# default -- without this, both real access paths 403 ("Rejected request
# with invalid Host header"): the browser's ingress hostname
# (mlflow.rezende.lab) and the in-cluster DNS name the 6 traced services'
# MLFLOW_TRACKING_URI actually uses (mlflow.rootcauseway.svc.cluster.local, plus
# the bare "mlflow" short name resolvable within the same namespace).
#
# --cors-allowed-origins: a SEPARATE MLflow 3.5+ security middleware from
# --allowed-hosts above -- fixing the Host-header check alone still left
# the UI's own same-origin XHRs 403'ing ("Blocked cross-origin request
# from https://mlflow.rezende.lab") because the CORS origin allowlist
# defaults to none of our real origins either.
CMD ["sh", "-c", "mlflow server \
    --host 0.0.0.0 \
    --port 5000 \
    --workers 2 \
    --backend-store-uri ${MLFLOW_BACKEND_STORE_URI} \
    --default-artifact-root mlflow-artifacts:/ \
    --artifacts-destination /mlflow-artifacts \
    --serve-artifacts \
    --allowed-hosts mlflow.rezende.lab,mlflow.rootcauseway.svc.cluster.local,mlflow.rootcauseway.svc.cluster.local:5000,mlflow,mlflow:5000,localhost:* \
    --cors-allowed-origins https://mlflow.rezende.lab,http://mlflow.rootcauseway.svc.cluster.local:5000,http://localhost:5000"]
