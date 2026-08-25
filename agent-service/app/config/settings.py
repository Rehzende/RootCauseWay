"""Application settings via pydantic-settings."""

from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    redis_url: str = "redis://localhost:6379/0"
    backend_api_url: str = "http://localhost:8080/api/v1"
    openai_api_key: str = "lm-studio"
    openai_api_base: str = "http://127.0.0.1:1234/v1"
    anthropic_api_key: str = ""
    llm_model: str = "qwen/qwen2.5-coder-14b"
    log_level: str = "INFO"

    # GenAI/LLM tracing (see mlflow-server/) -- same experiment name used by
    # every RootCauseway service so a stage-by-stage view of one incident's pipeline
    # is browsable in one place in the MLflow UI.
    mlflow_tracking_uri: str = "http://mlflow.rootcauseway.svc.cluster.local:5000"
    mlflow_experiment_name: str = "rootcauseway-incident-pipeline"
    # Public, browser-reachable URL for the "view trace" link persisted as
    # incident evidence -- deliberately separate from mlflow_tracking_uri
    # (in-cluster DNS, unreachable from a human's browser).
    mlflow_public_url: str = "https://mlflow.rezende.lab"

    # Redis Streams event consumption (see contracts/events/redis-events.yaml)
    event_stream_name: str = "rootcauseway:events"
    event_consumer_group: str = "agent-service"
    event_dlq_stream: str = "rootcauseway:events:dlq"
    event_dlq_maxlen: int = 10000
    event_max_retries: int = 3
    event_retry_backoff_base: float = 1.0  # seconds; delay = base * 2^attempt
    event_autoclaim_idle_ms: int = 60000  # claim pending entries idle longer than this
    # Backpressure: a single shared local LLM is the real bottleneck (not
    # consumer count -- more replicas just contend for the same LM Studio
    # instance), so under sustained load the stream can back up by minutes
    # per event. An event older than this by the time it's actually
    # dequeued is skipped rather than burning 1-3 minutes of LLM capacity
    # on something that (if it was ever real) has likely already resolved
    # or been superseded. 0 disables the check.
    stale_event_threshold_seconds: int = 900

    # A2A agent URLs (fallback when discovery from backend is unavailable)
    a2a_triage_agent_url: str = "http://triage-agent:8090"
    a2a_evidence_agent_url: str = "http://evidence-agent:8091"
    a2a_rca_agent_url: str = "http://rca-agent:8092"
    a2a_postmortem_agent_url: str = "http://postmortem-agent:8093"

    # A2A resilience (retry + circuit breaker)
    a2a_retry_attempts: int = 3
    a2a_retry_base_delay_seconds: float = 1.0
    a2a_breaker_threshold: int = 5
    a2a_breaker_cooldown_seconds: float = 30.0
    a2a_request_timeout_seconds: float = 120.0

    # War Room transcript summarizer: consumes warroom.meeting.ended off the
    # same rootcauseway:events stream as AlertWorker, but via its own consumer group
    # so it doesn't compete for/ack AlertWorker's entries.
    warroom_consumer_group: str = "warroom-service"
    warroom_poll_interval_ms: int = 1000
    warroom_autoclaim_idle_ms: int = 60000

    # Correlation engine: dependency-graph cascades + fingerprint dedup
    correlation_time_window_seconds: int = 300  # default same-service correlation window
    # 1h floor for a flapping alert's dedup window -- the backend query also
    # matches regardless of window when the prior incident is still
    # unresolved (see PgIncidentRepository.FindByFingerprint), so this value
    # only matters for how far back to look once that incident IS resolved.
    correlation_dedup_window_seconds: int = 3600

    model_config = {"env_prefix": "", "env_file": ".env", "extra": "ignore"}


def get_settings() -> Settings:
    return Settings()
