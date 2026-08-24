"""Inner loop: re-evaluates agent results and refines if confidence is low."""

from __future__ import annotations

import logging
from typing import Any

logger = logging.getLogger(__name__)


class InnerLoop:
    """Re-evaluates agent results and refines if confidence is low."""

    CONFIDENCE_THRESHOLD = 0.7

    def __init__(self) -> None:
        # Populated by evaluate_and_refine() after each run, since the
        # return value must stay exactly the results dict (callers/tests
        # rely on `refined == results` when no refinement fires) -- this
        # is the same "read an instance attribute after the call" pattern
        # Orchestrator.last_pipeline_timings already uses for out-of-band
        # data. Meant to be logged as MLflow span attributes by the
        # caller, to build up a labeled dataset (did this incident need
        # refinement, and how much) for a future predictive model -- see
        # the "meta-model of confidence" backlog item.
        self.last_run_stats: dict[str, Any] | None = None

    async def evaluate_and_refine(
        self,
        orchestrator: Any,
        incident_id: Any,
        results: dict[str, Any],
        max_iterations: int = 3,
    ) -> dict[str, Any]:
        """Check RCA confidence and re-run if below threshold.

        1. Check RCA confidence from results
        2. If confidence < 0.7, request more evidence and re-run RCA
        3. Repeat up to max_iterations
        4. Return final refined results
        """
        current_results = results
        initial_confidence = self._extract_confidence(results)
        iterations_run = 0
        final_confidence = initial_confidence

        for iteration in range(max_iterations):
            confidence = self._extract_confidence(current_results)
            logger.info(
                "Inner loop iteration %d/%d for incident %s: confidence=%.2f",
                iteration + 1, max_iterations, incident_id, confidence,
            )

            if confidence >= self.CONFIDENCE_THRESHOLD:
                logger.info(
                    "Confidence %.2f >= %.2f, skipping refinement",
                    confidence, self.CONFIDENCE_THRESHOLD,
                )
                final_confidence = confidence
                break

            # Request additional evidence and re-run via orchestrator
            logger.info(
                "Confidence %.2f < %.2f, requesting refinement (iteration %d)",
                confidence, self.CONFIDENCE_THRESHOLD, iteration + 1,
            )

            try:
                refinement_input = self._build_refinement_input(current_results, iteration)
                refined = await orchestrator.refine_rca(incident_id, refinement_input)
                iterations_run += 1
                if refined:
                    current_results = self._merge_results(current_results, refined)
                final_confidence = self._extract_confidence(current_results)
            except Exception:
                logger.warning(
                    "Refinement iteration %d failed for incident %s",
                    iteration + 1, incident_id,
                )
                final_confidence = self._extract_confidence(current_results)
                break
        else:
            final_confidence = self._extract_confidence(current_results)

        self.last_run_stats = {
            "initial_confidence": initial_confidence,
            "final_confidence": final_confidence,
            "iterations_run": iterations_run,
            "refined": iterations_run > 0,
        }
        return current_results

    def _extract_confidence(self, results: dict[str, Any]) -> float:
        """Extract RCA confidence score from results.

        Returns 1.0 if no RCA data exists (nothing to refine).
        """
        rca = results.get("rca")
        if not rca or not isinstance(rca, dict) or "error" in rca:
            # No valid RCA to refine
            return 1.0
        # Direct confidence field
        if "confidence" in rca:
            return float(rca["confidence"])
        # Nested inside rca artifact
        inner_rca = rca.get("rca", {})
        if isinstance(inner_rca, dict) and "confidence" in inner_rca:
            return float(inner_rca["confidence"])
        # RCA exists but no confidence field -- treat as needing refinement
        return 0.0

    def _build_refinement_input(
        self, results: dict[str, Any], iteration: int
    ) -> dict[str, Any]:
        """Build input for a refinement pass."""
        return {
            "previous_results": results,
            "refinement_iteration": iteration + 1,
            "instruction": (
                "The previous RCA has low confidence. Please gather additional "
                "evidence and refine the root cause analysis. Focus on areas "
                "where evidence was weak or inconclusive."
            ),
        }

    def _merge_results(
        self, original: dict[str, Any], refined: dict[str, Any]
    ) -> dict[str, Any]:
        """Merge refined results into original, preferring refined values."""
        merged = {**original}
        for key, value in refined.items():
            if isinstance(value, dict) and isinstance(merged.get(key), dict):
                merged[key] = {**merged[key], **value}
            else:
                merged[key] = value
        return merged
