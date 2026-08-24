package handlers

import "testing"

// TestIsOrchestratorWrittenEvidence pins the fix for a live-found infinite
// re-analysis loop: AddIncidentEvidence used to gate its evidence.uploaded
// publish on a hardcoded list of 4 literal sources
// ("agent:triage"/"agent:evidence-collection"/"agent:rca"/"agent:postmortem"),
// which only matched the 4 built-in agents' skill IDs. A custom skill's
// source is "agent:<uuid>" -- never in that list -- so every evidence write
// for a custom skill republished evidence.uploaded, which re-triggered
// analysis, which wrote more evidence, forever. The "mlflow" trace-link
// source was never excluded at all, on any skill. Confirmed live: the same
// incident re-analyzed 4 times in under 5 minutes before this was caught.
func TestIsOrchestratorWrittenEvidence(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   bool
	}{
		{"built-in triage skill", "agent:triage", true},
		{"built-in evidence skill", "agent:evidence-collection", true},
		{"built-in rca skill", "agent:rca", true},
		{"built-in postmortem skill", "agent:postmortem", true},
		{"custom skill by uuid", "agent:4ac73e02-5cdb-4e96-9c74-fecd8aca3926", true},
		{"mlflow trace link", "mlflow", true},
		{"human-uploaded file", "user-upload", false},
		{"prometheus snapshot collector", "prometheus", false},
		{"empty source", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isOrchestratorWrittenEvidence(tc.source); got != tc.want {
				t.Errorf("isOrchestratorWrittenEvidence(%q) = %v, want %v", tc.source, got, tc.want)
			}
		})
	}
}
