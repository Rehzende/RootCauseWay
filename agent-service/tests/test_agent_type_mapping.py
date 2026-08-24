"""Regression test for _AGENT_TYPE_BY_SKILL.

A real load test found agent_run creation 500ing for the "incident-analysis"
skill (agent_runs.agent_type CHECK constraint violation, see
backend/migrations/002_incident_cockpit.up.sql). rootcauseway_swallowed_errors_total
caught it live in production. Investigating showed 3 more of k8s-agent's
skills (k8s-debug, k8s-logs, k8s-diagnostics) had the exact same latent bug
-- just never picked by the LLM's skill selection yet. This test guards
every skill_id any AgentCard in this repo actually advertises against the
backend's real CHECK constraint values, not just the 4 that happened to be
exercised so far.

A second pass (validating the Skills registry fix live) found the same bug
class again, worse: a user-created custom skill's skill_id is a UUID (e.g.
"4ac73e02-5cdb-4e96-9c74-fecd8aca3926"), and the old fallback
(`skill_id.replace("-", "_")`) turned that into
"4ac73e02_5cdb_4e96_9c74_fecd8aca3926" -- nowhere close to a valid
agent_type, 500ing every single custom skill dispatch's agent_run creation
(the actual A2A dispatch/analysis still succeeded; only the DAG/cockpit
tracking row was lost). Fallback changed to the literal "custom" value the
CHECK constraint already reserves for exactly this case.
"""

from __future__ import annotations

from app.orchestrator.orchestrator import _AGENT_TYPE_BY_SKILL

# backend/migrations/002_incident_cockpit.up.sql:
#   agent_type VARCHAR(30) NOT NULL CHECK (agent_type IN
#     ('triage', 'evidence_analysis', 'hypothesis', 'rci_generator',
#      'rca_generator', 'postmortem_generator', 'debug', 'custom'))
VALID_AGENT_TYPES = {
    "triage", "evidence_analysis", "hypothesis", "rci_generator",
    "rca_generator", "postmortem_generator", "debug", "custom",
}

# Every skill_id an AgentCard in agents/*/app/main.py actually advertises.
# Keep this in sync when a new skill is added to any agent's card -- that's
# exactly the kind of addition that silently reintroduces this bug class.
KNOWN_SKILL_IDS = {
    "triage",
    "evidence-collection",
    "rca",
    "postmortem",
    "k8s-debug",
    "k8s-logs",
    "k8s-diagnostics",
    "incident-analysis",
}


def test_all_known_skills_are_mapped():
    unmapped = KNOWN_SKILL_IDS - _AGENT_TYPE_BY_SKILL.keys()
    assert not unmapped, (
        f"skill_id(s) {unmapped} have no _AGENT_TYPE_BY_SKILL entry -- "
        "the orchestrator.py fallback (skill_id.replace('-', '_')) only "
        "happens to produce a valid agent_type for 'triage'; every other "
        "skill 500s on agent_runs_agent_type_check the first time the LLM "
        "picks it."
    )


def test_all_mapped_agent_types_satisfy_the_check_constraint():
    invalid = {
        skill_id: agent_type
        for skill_id, agent_type in _AGENT_TYPE_BY_SKILL.items()
        if agent_type not in VALID_AGENT_TYPES
    }
    assert not invalid, f"mapped to a value the DB CHECK constraint rejects: {invalid}"


def test_k8s_agent_skills_map_to_debug():
    for skill_id in ("k8s-debug", "k8s-logs", "k8s-diagnostics", "incident-analysis"):
        assert _AGENT_TYPE_BY_SKILL[skill_id] == "debug"


def test_unmapped_skill_id_falls_back_to_custom_not_a_mangled_uuid():
    """The exact fallback expression orchestrator.py's create_agent_run
    call site uses -- a custom skill's skill_id is a UUID, and the old
    fallback (skill_id.replace("-", "_")) produced a CHECK-constraint-
    violating value for every single one."""
    custom_skill_id = "4ac73e02-5cdb-4e96-9c74-fecd8aca3926"
    fallback = _AGENT_TYPE_BY_SKILL.get(custom_skill_id, "custom")
    assert fallback == "custom"
    assert fallback in VALID_AGENT_TYPES
