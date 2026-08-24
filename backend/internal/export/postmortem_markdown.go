package export

import (
	"fmt"
	"strings"

	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// RenderMarkdown renders a postmortem as a clean, well-structured Markdown
// document suitable for pasting into Confluence/Notion or sending by email.
// incident may be nil (e.g. lookup failed upstream); when present it is used
// to enrich the header with title/severity/status context.
func RenderMarkdown(postmortem *models.IncidentPostmortem, incident *models.Incident) (string, error) {
	if postmortem == nil {
		return "", fmt.Errorf("export: postmortem is nil")
	}

	var b strings.Builder

	title := postmortem.Title
	if title == "" {
		title = "Postmortem"
	}
	b.WriteString("# " + title + "\n\n")

	// Metadata line(s).
	if incident != nil {
		b.WriteString("**Incident:** " + nonEmpty(incident.Title, incident.ID.String()) + "  \n")
		if incident.Severity != "" {
			b.WriteString("**Severity:** " + incident.Severity + "  \n")
		}
		if incident.Status != "" {
			b.WriteString("**Status:** " + incident.Status + "  \n")
		}
		b.WriteString("**Incident ID:** " + incident.ID.String() + "  \n")
	} else {
		b.WriteString("**Incident ID:** " + postmortem.IncidentID.String() + "  \n")
	}
	if postmortem.Status != "" {
		b.WriteString("**Postmortem Status:** " + postmortem.Status + "  \n")
	}
	if !postmortem.CreatedAt.IsZero() {
		b.WriteString("**Generated:** " + postmortem.CreatedAt.Format("2006-01-02") + "  \n")
	}
	b.WriteString("\n")

	writeSection(&b, "Executive Summary", postmortem.ExecutiveSummary)
	writeSection(&b, "Incident Timeline", postmortem.IncidentTimelineNarrative)
	writeSection(&b, "Root Cause", postmortem.RootCauseDetail)
	writeSection(&b, "Impact", postmortem.ImpactDetail)

	writeBulletSection(&b, "Lessons Learned", normalizeStringList(postmortem.LessonsLearned))
	writeBulletSection(&b, "What Went Well", normalizeStringList(postmortem.WhatWentWell))
	writeBulletSection(&b, "What Went Wrong", normalizeStringList(postmortem.WhatWentWrong))

	writeActionItemsSection(&b, normalizeActionItems(postmortem.ActionItems))

	writeBulletSection(&b, "Prevention Measures", normalizeStringList(postmortem.PreventionMeasures))

	return b.String(), nil
}

func nonEmpty(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func writeSection(b *strings.Builder, heading, body string) {
	b.WriteString("## " + heading + "\n\n")
	if body == "" {
		b.WriteString("_Not documented._\n\n")
		return
	}
	b.WriteString(body + "\n\n")
}

func writeBulletSection(b *strings.Builder, heading string, items []string) {
	b.WriteString("## " + heading + "\n\n")
	if len(items) == 0 {
		b.WriteString("_None recorded._\n\n")
		return
	}
	for _, item := range items {
		b.WriteString("- " + item + "\n")
	}
	b.WriteString("\n")
}

func writeActionItemsSection(b *strings.Builder, items []actionItem) {
	b.WriteString("## Action Items\n\n")
	if len(items) == 0 {
		b.WriteString("_None recorded._\n\n")
		return
	}
	b.WriteString("| Done | Description | Owner | Priority |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, item := range items {
		desc := item.Title
		if item.Description != "" {
			if desc != "" {
				desc += " — " + item.Description
			} else {
				desc = item.Description
			}
		}
		desc = escapeTableCell(desc)
		owner := escapeTableCell(nonEmpty(item.Assignee, "_unassigned_"))
		priority := escapeTableCell(nonEmpty(item.Priority, "-"))
		b.WriteString("| [ ] | " + desc + " | " + owner + " | " + priority + " |\n")
	}
	b.WriteString("\n")
}

func escapeTableCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
