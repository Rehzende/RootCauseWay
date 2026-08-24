"""
Inner loop: re-runs RCA if confidence < 0.7 (max 3 iterations).
Outer loop: extracts lessons from resolved incidents into knowledge base.
"""
import httpx
import logging
from typing import Optional

logger = logging.getLogger(__name__)

BACKEND_URL = "http://localhost:8080"

class LoopEngine:
    def __init__(self, backend_token: str):
        self.backend_token = backend_token
        self.headers = {"Authorization": f"Bearer {backend_token}", "Content-Type": "application/json"}

    async def evaluate_confidence(self, rca_result: dict) -> float:
        """Extract confidence score from RCA result text."""
        text = str(rca_result.get("output", ""))
        if "high confidence" in text.lower():
            return 0.9
        if "medium confidence" in text.lower() or "likely" in text.lower():
            return 0.7
        if "uncertain" in text.lower() or "unclear" in text.lower() or "insufficient" in text.lower():
            return 0.4
        # Default moderate confidence
        return 0.65

    async def inner_loop(self, incident_id: str, orchestrator, max_iterations: int = 3) -> dict:
        """Re-run analysis if confidence is low. Returns final result."""
        result = {}
        for iteration in range(max_iterations):
            logger.info(f"Loop iteration {iteration+1}/{max_iterations} for incident {incident_id}")
            result = await orchestrator.run_pipeline(incident_id, iteration=iteration)
            confidence = await self.evaluate_confidence(result)
            logger.info(f"Confidence: {confidence:.2f}")
            if confidence >= 0.7:
                break
            if iteration < max_iterations - 1:
                logger.info(f"Low confidence ({confidence:.2f}), requesting more evidence...")
                # Publish evidence request — agent-service will handle re-collection
                await self._request_more_evidence(incident_id)
        return result

    async def _request_more_evidence(self, incident_id: str):
        try:
            async with httpx.AsyncClient() as client:
                await client.post(
                    f"{BACKEND_URL}/api/v1/incidents/{incident_id}/evidence",
                    json={"type": "loop_rerun", "content": "Requesting additional evidence for low-confidence RCA", "source": "loop_engine"},
                    headers=self.headers,
                    timeout=10
                )
        except Exception as e:
            logger.error(f"Failed to request more evidence: {e}")

    async def extract_lessons(self, incident_id: str, org_id: str, rca_output: str, software_id: Optional[str] = None):
        """Outer loop: extract root cause and resolution from RCA, store in knowledge base."""
        # Simple extraction — look for key sections in RCA text
        root_cause = self._extract_section(rca_output, ["root cause:", "root_cause:", "## root cause"])
        resolution = self._extract_section(rca_output, ["resolution:", "## resolution", "fix:", "remediation:"])
        lessons = self._extract_section(rca_output, ["lessons learned:", "## lessons", "key takeaways:"])

        if not root_cause or not resolution:
            logger.info(f"Could not extract lessons from incident {incident_id}")
            return

        payload = {
            "root_cause": root_cause[:500],
            "resolution": resolution[:500],
            "lessons_learned": lessons[:500] if lessons else None,
            "source_incident_id": incident_id,
            "human_validated": False,
        }
        if software_id:
            payload["software_id"] = software_id

        try:
            async with httpx.AsyncClient() as client:
                resp = await client.post(
                    f"{BACKEND_URL}/api/v1/knowledge-base",
                    json=payload,
                    headers={**self.headers, "X-Org-ID": org_id},
                    timeout=10
                )
                if resp.status_code == 201:
                    logger.info(f"Knowledge base entry created for incident {incident_id}")
        except Exception as e:
            logger.error(f"Failed to create knowledge base entry: {e}")

    def _extract_section(self, text: str, markers: list) -> Optional[str]:
        text_lower = text.lower()
        for marker in markers:
            idx = text_lower.find(marker)
            if idx >= 0:
                start = idx + len(marker)
                # Take up to 500 chars or next section
                snippet = text[start:start+600].strip()
                # Stop at next heading
                for stop in ["\n##", "\n**", "\n\n\n"]:
                    stop_idx = snippet.find(stop)
                    if stop_idx > 0:
                        snippet = snippet[:stop_idx]
                return snippet.strip()
        return None
