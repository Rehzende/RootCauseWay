// Package export renders IncidentPostmortem records into shareable document
// formats (Markdown, PDF) so they can be pulled out of the platform for
// sharing in Confluence/Notion/email. Both renderers consume the same
// normalized data — see actionItem/normalizeStringList/normalizeActionItems
// below — and only differ in how they lay it out.
package export

import (
	"encoding/json"
)

// actionItem is the normalized shape of a single postmortem action item.
// The DB/agent-service stores IncidentPostmortem.ActionItems as a JSON blob
// shaped like agent-service's ActionItem model (title, description,
// priority, assignee); we tolerate missing fields and non-object entries.
type actionItem struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Assignee    string `json:"assignee"`
}

// normalizeStringList best-effort decodes a json.RawMessage that is expected
// to hold a JSON array of strings (e.g. lessons_learned, what_went_well).
// Nil, empty, or malformed input yields an empty (non-nil) slice rather than
// an error or panic, since export is a read-only, best-effort view.
func normalizeStringList(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var items []string
	if err := json.Unmarshal(raw, &items); err != nil {
		return []string{}
	}
	// Drop blank entries.
	out := make([]string, 0, len(items))
	for _, s := range items {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// normalizeActionItems best-effort decodes the action_items JSON blob into a
// slice of actionItem. Malformed/empty input yields an empty slice.
func normalizeActionItems(raw json.RawMessage) []actionItem {
	if len(raw) == 0 {
		return []actionItem{}
	}
	var items []actionItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return []actionItem{}
	}
	return items
}
