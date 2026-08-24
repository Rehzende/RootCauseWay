"""Runbook executor: drives a runbook execution's "automated" (and
"condition"/"notification") steps forward, stopping the moment it reaches a
"manual" or "approval" step -- those still need a human, via the existing
Mark Complete / approve UI.

This class existed before but was never imported anywhere, and its
top-level loop called BackendClient.update_runbook_execution with an ad-hoc
payload shape ("step_status", "waiting_step") that never matched the real
Go model (UpdateRunbookExecutionRequest: status/current_step/step_results
only) -- it would have 400'd/silently-no-op'd on every real call even if it
had been wired up. Rewritten to drive state through
POST .../steps/:stepId/complete instead, the same endpoint the UI's "Mark
Complete" button already uses and that already has correct, tested
advancement logic (see FeaturesHandler.CompleteExecutionStep) -- rather than
re-implementing that state machine a second time in Python.
"""

from __future__ import annotations

import logging
from typing import Any
from uuid import UUID, uuid4

from app.credentials.jit_provider import JITCredentialProvider
from app.services.backend_client import BackendClient

logger = logging.getLogger(__name__)

# Statuses CompleteExecutionStep sets on the step it just advanced *to*
# (see its switch on step_type) that mean "ready to run now, no human
# needed" -- "running" for automated/condition/notification/anything else,
# vs "pending_action"/"pending_approval" for manual/approval.
_READY_STATUS = "running"


class RunbookExecutor:
    """Drives a runbook execution's auto-processable steps forward."""

    async def run_automated_steps(
        self,
        backend_client: BackendClient,
        jit_provider: JITCredentialProvider,
        orchestrator: Any,
        org_id: UUID,
        execution_id: UUID,
        incident_id: UUID,
    ) -> dict[str, Any]:
        """Process steps in order starting from wherever the execution
        currently is, until hitting a manual/approval step or a terminal
        execution status. Returns the execution's latest state."""
        execution = await backend_client.get_runbook_execution(execution_id, org_id)
        runbook_id = execution.get("runbook_id", "")
        try:
            runbook = await backend_client.get_runbook(runbook_id, org_id)
            steps = await backend_client.list_runbook_steps(runbook_id, org_id)
        except Exception:
            logger.exception(
                "Failed to fetch runbook %s (execution %s) -- "
                "cannot auto-process any steps", runbook_id, execution_id,
            )
            return execution
        # A runbook step's own row carries only skill_id -- no agent_url,
        # software_id, or org_id, all of which execute_automated_step needs
        # to actually dispatch. agent_url is resolved the same way the main
        # orchestrator resolves it for a normal incident dispatch: look up
        # the skill's linked agent by skill_id. software_id/org_id come
        # from the parent runbook/this call's own org_id.
        try:
            skills = await backend_client.list_skills(org_id)
        except Exception:
            logger.warning("Failed to list skills while resolving runbook step agents", exc_info=True)
            skills = []
        agent_url_by_skill = {
            s.get("id"): s["agents"][0]["url"]
            for s in skills if s.get("agents")
        }
        software_id = runbook.get("software_id") or ""
        steps_by_id = {
            str(s.get("id")): {
                **s,
                "software_id": software_id,
                "org_id": org_id,
                "agent_url": agent_url_by_skill.get(str(s.get("skill_id", ""))),
            }
            for s in steps
        }

        # Safety valve: bound iterations to the step count so a state-
        # machine bug (or a condition step's on_failure looping back)
        # can't spin forever burning agent calls.
        for _ in range(len(steps) + 1):
            if execution.get("status") in ("completed", "completed_with_errors"):
                break

            current = _find_ready_step(execution.get("step_results") or [])
            if current is None:
                break

            step_id = current.get("step_id", "")
            step_type = current.get("step_type", "")
            step_meta = steps_by_id.get(step_id, {})

            if step_type == "automated":
                result = await self.execute_automated_step(
                    backend_client, jit_provider, orchestrator, step_meta, incident_id,
                )
            elif step_type == "condition":
                result = await self._handle_condition_step(step_meta, execution.get("step_results") or [])
            elif step_type == "notification":
                result = await self._handle_notification_step(step_meta, incident_id)
            else:
                # manual, approval, or an unrecognized type -- needs a
                # human, or there's nothing sensible to automate; stop.
                break

            execution = await backend_client.complete_runbook_execution_step(
                execution_id, step_id, org_id,
                status=result.get("status", "completed"), output=result,
            )

        return execution

    async def execute_automated_step(
        self,
        backend_client: BackendClient,
        jit_provider: JITCredentialProvider,
        orchestrator: Any,
        step: dict[str, Any],
        incident_id: UUID,
    ) -> dict[str, Any]:
        """Execute an automated step using the skill linked to the step."""
        skill_id = step.get("skill_id", "")
        agent_url = step.get("agent_url", "")
        resource_types = step.get("required_resource_types", [])

        credentials = {}
        lease_ids = []
        for resource_type in resource_types:
            try:
                lease = await jit_provider.request_credentials(
                    incident_id=incident_id,
                    agent_id=uuid4(),
                    skill_id=skill_id,
                    software_id=step.get("software_id", ""),
                    resource_type=resource_type,
                    org_id=step.get("org_id"),
                    ttl_seconds=900,
                    reason=f"Runbook step: {step.get('name', skill_id)}",
                )
                if lease:
                    credentials[resource_type] = lease.credential_data
                    lease_ids.append(lease.id)
            except Exception:
                logger.warning("Failed to get JIT credentials for runbook step %s", skill_id)

        try:
            result = await orchestrator.dispatch_single_skill(
                incident_id=incident_id,
                skill_id=skill_id,
                agent_url=agent_url,
                input_data=step.get("input", {}),
                credentials=credentials,
            )
            return {"status": "completed", "result": result}
        except Exception as exc:
            return {"status": "failed", "error": str(exc)}
        finally:
            for lid in lease_ids:
                try:
                    await jit_provider.revoke_credentials(lid)
                except Exception:
                    logger.warning("Failed to revoke lease %s", lid)

    async def _handle_notification_step(
        self, step: dict[str, Any], incident_id: UUID,
    ) -> dict[str, Any]:
        """Send a notification as part of the runbook. Not wired to the
        real NotificationDispatcher yet -- same scope boundary as before,
        left as a completed no-op rather than blocking the rest of the
        runbook on an unbuilt integration."""
        return {"status": "completed", "message": "Notification step acknowledged (dispatch not yet wired)"}

    async def _handle_condition_step(
        self, step: dict[str, Any], previous_results: list[dict[str, Any]]
    ) -> dict[str, Any]:
        """Evaluate a condition based on previous step results."""
        condition = step.get("condition", {})
        check_step = condition.get("check_step_id")
        expected_status = condition.get("expected_status", "completed")

        if check_step:
            matching = [r for r in previous_results if r.get("step_id") == check_step]
            if matching and matching[-1].get("status") == expected_status:
                return {"status": "completed", "action": "continue", "condition_met": True}
            else:
                action = condition.get("on_failure", "continue")
                return {"status": "completed", "action": action, "condition_met": False}

        return {"status": "completed", "action": "continue", "condition_met": True}


def _find_ready_step(step_results: list[dict[str, Any]]) -> dict[str, Any] | None:
    """The step CompleteExecutionStep (or ExecuteRunbook, for the very
    first step) most recently marked "running" -- i.e. next in line and not
    waiting on a human. None if no such step exists (execution just
    started, is stuck on manual/approval, or is done)."""
    for sr in step_results:
        if sr.get("status") == _READY_STATUS:
            return sr
    return None
