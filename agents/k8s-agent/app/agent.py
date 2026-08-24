"""K8s Debug Agent — collects kubectl data and, when given a `service` to
investigate, correlates it with Prometheus/Loki/Tempo/Alertmanager and asks
an LLM for a root-cause analysis.

Backward compatible: if the incoming task only has {namespace, pod, labels}
(no `service`), it behaves exactly like before — pure data collection, no LLM
call. The RCA path only activates when the caller explicitly names a service
to investigate, e.g. from an Alertmanager webhook or another RootCauseway agent.
"""

from __future__ import annotations

import asyncio
import json
import logging
import os
import shutil
import tempfile
from typing import Any

import httpx
import mlflow
from mlflow.entities import SpanType

from app import observability
from app.a2a.models import Artifact, DataPart, Message, Task, TaskStatus
from app.llm import analyze_incident

logger = logging.getLogger(__name__)

KUBECTL_TIMEOUT = 30


class K8sDebugAgent:
    """Collects Kubernetes + observability data and optionally runs an
    LLM-backed root-cause analysis on top of it."""

    def __init__(self, api_base: str = "", api_key: str = "", model: str = "", **_kwargs: Any):
        self._in_cluster = os.path.exists("/var/run/secrets/kubernetes.io/serviceaccount/token")
        self._api_base = api_base
        self._api_key = api_key
        self._model = model

    # KNOWN GAP (found live validating cross-agent trace propagation, not
    # caused by it): unlike the other 4 agents, this one's own outbound
    # Tempo tracing (app/telemetry.py's setup_telemetry(), FastAPIInstrumentor
    # + HTTPXClientInstrumentor + its own OTel TracerProvider) coexists with
    # MLflow's isolated-mode tracer in the same process, and the two collide
    # somewhere in that stack -- get_current_active_span() reads None from
    # inside this very function body, so no k8s_agent.* span (this one,
    # analyze_service, or analyze_incident) has ever actually reached
    # MLflow, confirmed live and independent of the propagation fix (no
    # "k8s_agent.*" trace exists in MLflow at all, historical or fresh, via
    # the orchestrator, the A2A mesh, or the standalone Alertmanager webhook
    # path). The decorator is correct and worth keeping -- it'll start
    # working the moment the OTel collision is untangled -- but that's a
    # separate, deeper investigation than this fix's scope.
    @mlflow.trace(span_type=SpanType.AGENT, name="k8s_agent.handle_task")
    async def handle_task(self, task_id: str, message: Message) -> Task:
        """
        Input: {namespace, pod, labels, kubeconfig (optional)}
               + optionally {service, otel_service} to trigger RCA

        Output artifacts:
          - k8s_cluster_data: raw kubectl output (always)
          - root_cause_analysis: LLM-generated RCA (only if `service` given)
        """
        input_data = self._extract_data(message)
        alert = input_data.get("alert") or {}
        # Found live validating the Azure lab chaos pipeline: Go's
        # NormalizedAlert (backend/internal/models/models.go) promotes
        # `service` to a top-level field on the alert (normalizer copies it
        # up from the label), but never does the same for `namespace` --
        # that one only ever exists nested inside the label map. And on
        # the Python side (agent-service/app/models/events.py), the
        # NormalizedAlert Pydantic model doesn't even declare a `labels`
        # field -- only `tags` -- so Go's `Labels: alert.Labels` value is
        # silently dropped by Pydantic's model_validate and never reaches
        # this agent at all; only `Tags: alert.Labels` (the same data,
        # published under the OTHER field name) survives the round trip.
        # Checking `alert.labels.namespace` (confirmed live: still empty)
        # was the wrong nesting -- `alert.tags.namespace` is the one that
        # actually carries it. Getting this wrong meant every kubectl call
        # below queried the wrong namespace and came back empty -- the RCA
        # agent then confidently fabricated a root cause from zero real
        # evidence, twice.
        namespace = (
            input_data.get("namespace")
            or alert.get("namespace")
            or (alert.get("tags") or {}).get("namespace")
            or (alert.get("labels") or {}).get("namespace")
            or "default"
        )
        pod = input_data.get("pod", alert.get("pod", ""))
        service = input_data.get("service", alert.get("service", ""))
        kubeconfig = input_data.get("credentials", {}).get("kubeconfig", "")
        labels = dict(input_data.get("labels", {}))

        if not shutil.which("kubectl"):
            return Task(
                id=task_id,
                status=TaskStatus.FAILED,
                artifacts=[Artifact(
                    name="error",
                    description="kubectl not found",
                    parts=[DataPart(data={"error": "kubectl not available in this environment"})],
                )],
            )

        # If we're investigating a named service and no explicit pod/labels
        # were given, default the label selector to app=<service> so kubectl
        # data collection targets the right pods automatically.
        if service and not pod and not labels:
            labels = {"app": service}

        data = await self._collect(namespace, pod, labels, kubeconfig)

        artifacts = [
            Artifact(
                name="k8s_cluster_data",
                description=f"Raw kubectl output for namespace={namespace}",
                parts=[DataPart(data=data)],
            )
        ]

        if service:
            otel_service = input_data.get("otel_service", service)
            rca = await self.analyze_service(namespace, service, otel_service, data)
            artifacts.append(
                Artifact(
                    name="root_cause_analysis",
                    description=f"LLM root-cause analysis for service={service}",
                    parts=[DataPart(data=rca)],
                )
            )

        return Task(id=task_id, status=TaskStatus.COMPLETED, artifacts=artifacts)

    @mlflow.trace(span_type=SpanType.CHAIN, name="k8s_agent.analyze_service")
    async def analyze_service(
        self, namespace: str, service: str, otel_service: str | None = None, k8s_data: dict | None = None
    ) -> dict[str, Any]:
        """Correlate Prometheus/Loki/Tempo/Alertmanager evidence for `service`
        and hand it to the LLM for a structured RCA.

        `k8s_data` lets `handle_task` pass along kubectl output it already
        collected for this request; callers that only have a service name
        (e.g. the alert webhook) can omit it and it'll be collected here.
        """
        otel_service = otel_service or service
        if k8s_data is None:
            k8s_data = await self._collect(namespace, pod="", labels={"app": service}, kubeconfig="")

        async with httpx.AsyncClient() as client:
            metrics, alerts, logs, traces = await asyncio.gather(
                observability.collect_prometheus_evidence(client, job=service, namespace=namespace, pod_prefix=service),
                observability.collect_active_alerts(client, service=service),
                observability.collect_recent_error_logs(client, namespace=namespace, app=service),
                observability.collect_recent_error_traces(client, otel_service=otel_service),
            )

        evidence = {
            "service": service,
            "namespace": namespace,
            "metrics": metrics,
            "active_alerts": alerts,
            "recent_error_logs": logs,
            "recent_error_traces": traces,
            "k8s_pod_status": k8s_data.get("pods_by_label") or k8s_data.get("pods"),
            "k8s_recent_events": k8s_data.get("events"),
        }

        if not self._api_base:
            return {"error": "No LLM configured (OPENAI_API_BASE unset)", "evidence": evidence}

        rca = await analyze_incident(
            evidence=evidence,
            api_base=self._api_base,
            api_key=self._api_key,
            model=self._model,
        )
        # Keep the raw evidence alongside the LLM's conclusions — useful for a
        # human reviewing the artifact, and lets a downstream agent redo the
        # analysis without re-querying every backend.
        rca["evidence"] = evidence
        return rca

    async def _collect(
        self, namespace: str, pod: str, labels: dict, kubeconfig: str
    ) -> dict[str, Any]:
        kubeconfig_path = None
        if kubeconfig:
            with tempfile.NamedTemporaryFile(mode="w", suffix=".yaml", delete=False) as f:
                f.write(kubeconfig)
                kubeconfig_path = f.name

        try:
            env = os.environ.copy()
            if kubeconfig_path:
                env["KUBECONFIG"] = kubeconfig_path
            # In-cluster: no KUBECONFIG needed — kubectl uses service account automatically

            label_selector = ",".join(f"{k}={v}" for k, v in labels.items()) if labels else ""

            commands: dict[str, str] = {
                "pods": f"kubectl get pods -n {namespace} -o wide",
                "events": f"kubectl get events -n {namespace} --sort-by=.lastTimestamp",
                "deployments": f"kubectl get deployments -n {namespace}",
                "nodes": "kubectl get nodes -o wide",
                "top_pods": f"kubectl top pods -n {namespace}",
            }

            if label_selector:
                commands["pods_by_label"] = f"kubectl get pods -n {namespace} -l {label_selector} -o wide"

            if pod:
                commands["describe_pod"] = f"kubectl describe pod {pod} -n {namespace}"
                commands["logs"] = f"kubectl logs {pod} -n {namespace} --tail=200 --timestamps"
                commands["logs_previous"] = f"kubectl logs {pod} -n {namespace} --tail=100 --previous"

            results: dict[str, Any] = {
                "namespace": namespace,
                "pod": pod,
                "in_cluster": self._in_cluster,
            }

            for key, cmd in commands.items():
                results[key] = await self._run(cmd, env)

            return results

        finally:
            if kubeconfig_path:
                os.unlink(kubeconfig_path)

    async def _run(self, cmd: str, env: dict) -> str:
        try:
            proc = await asyncio.create_subprocess_shell(
                cmd,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
                env=env,
            )
            stdout, stderr = await asyncio.wait_for(proc.communicate(), timeout=KUBECTL_TIMEOUT)
            if proc.returncode == 0:
                return stdout.decode("utf-8", errors="replace").strip()
            return f"[exit {proc.returncode}] {stderr.decode('utf-8', errors='replace').strip()}"
        except asyncio.TimeoutError:
            return "[timeout]"
        except Exception as exc:
            return f"[error] {exc}"

    @staticmethod
    def _extract_data(message: Message) -> dict[str, Any]:
        for part in message.parts:
            if hasattr(part, "data"):
                return part.data
        return {}
