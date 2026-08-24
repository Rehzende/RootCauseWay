package export

import (
	"bytes"
	"fmt"

	"github.com/go-pdf/fpdf"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// RenderPDF renders a postmortem as a PDF document, mirroring the section
// layout used by RenderMarkdown (executive summary, timeline, root cause,
// impact, lessons learned, what-went-well/wrong, action items, prevention
// measures). incident may be nil.
//
// Uses github.com/go-pdf/fpdf (the actively maintained successor to
// jung-kurt/gofpdf) because it is a pure-Go, dependency-free PDF generator —
// no external binary (wkhtmltopdf, headless Chrome) is required, which
// matters for reliability in this environment.
func RenderPDF(postmortem *models.IncidentPostmortem, incident *models.Incident) ([]byte, error) {
	if postmortem == nil {
		return nil, fmt.Errorf("export: postmortem is nil")
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(18, 18, 18)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddPage()

	title := postmortem.Title
	if title == "" {
		title = "Postmortem"
	}
	pdf.SetFont("Helvetica", "B", 20)
	pdf.MultiCell(0, 10, title, "", "L", false)
	pdf.Ln(2)

	pdf.SetFont("Helvetica", "", 10)
	if incident != nil {
		writeMeta(pdf, "Incident", nonEmpty(incident.Title, incident.ID.String()))
		if incident.Severity != "" {
			writeMeta(pdf, "Severity", incident.Severity)
		}
		if incident.Status != "" {
			writeMeta(pdf, "Status", incident.Status)
		}
		writeMeta(pdf, "Incident ID", incident.ID.String())
	} else {
		writeMeta(pdf, "Incident ID", postmortem.IncidentID.String())
	}
	if postmortem.Status != "" {
		writeMeta(pdf, "Postmortem Status", postmortem.Status)
	}
	if !postmortem.CreatedAt.IsZero() {
		writeMeta(pdf, "Generated", postmortem.CreatedAt.Format("2006-01-02"))
	}
	pdf.Ln(4)

	pdfSection(pdf, "Executive Summary", postmortem.ExecutiveSummary)
	pdfSection(pdf, "Incident Timeline", postmortem.IncidentTimelineNarrative)
	pdfSection(pdf, "Root Cause", postmortem.RootCauseDetail)
	pdfSection(pdf, "Impact", postmortem.ImpactDetail)

	pdfBulletSection(pdf, "Lessons Learned", normalizeStringList(postmortem.LessonsLearned))
	pdfBulletSection(pdf, "What Went Well", normalizeStringList(postmortem.WhatWentWell))
	pdfBulletSection(pdf, "What Went Wrong", normalizeStringList(postmortem.WhatWentWrong))

	pdfActionItems(pdf, normalizeActionItems(postmortem.ActionItems))

	pdfBulletSection(pdf, "Prevention Measures", normalizeStringList(postmortem.PreventionMeasures))

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("export: render pdf: %w", err)
	}
	return buf.Bytes(), nil
}

func writeMeta(pdf *fpdf.Fpdf, label, value string) {
	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(35, 6, label+":", "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.MultiCell(0, 6, value, "", "L", false)
}

func pdfHeading(pdf *fpdf.Fpdf, heading string) {
	pdf.Ln(2)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(0, 8, heading, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 11)
}

func pdfSection(pdf *fpdf.Fpdf, heading, body string) {
	pdfHeading(pdf, heading)
	if body == "" {
		pdf.SetFont("Helvetica", "I", 11)
		pdf.MultiCell(0, 6, "Not documented.", "", "L", false)
		return
	}
	pdf.MultiCell(0, 6, body, "", "L", false)
}

func pdfBulletSection(pdf *fpdf.Fpdf, heading string, items []string) {
	pdfHeading(pdf, heading)
	if len(items) == 0 {
		pdf.SetFont("Helvetica", "I", 11)
		pdf.MultiCell(0, 6, "None recorded.", "", "L", false)
		return
	}
	for _, item := range items {
		pdf.MultiCell(0, 6, "- "+item, "", "L", false)
	}
}

func pdfActionItems(pdf *fpdf.Fpdf, items []actionItem) {
	pdfHeading(pdf, "Action Items")
	if len(items) == 0 {
		pdf.SetFont("Helvetica", "I", 11)
		pdf.MultiCell(0, 6, "None recorded.", "", "L", false)
		return
	}
	for _, item := range items {
		desc := item.Title
		if item.Description != "" {
			if desc != "" {
				desc += " -- " + item.Description
			} else {
				desc = item.Description
			}
		}
		owner := nonEmpty(item.Assignee, "unassigned")
		priority := nonEmpty(item.Priority, "-")
		pdf.MultiCell(0, 6, fmt.Sprintf("[ ] %s (owner: %s, priority: %s)", desc, owner, priority), "", "L", false)
	}
}
