// Mirrors backend/internal/models.FormatIncidentCode: renders an incident's
// sequential per-org number as the human-friendly "INC-0001" display code.
// Zero-padded to 4 digits but not truncated -- number 12345 renders as
// "INC-12345", not clipped.
export function formatIncidentCode(incidentNumber: number): string {
  return `INC-${String(incidentNumber).padStart(4, '0')}`;
}

// "INC-0001 - Title", the standard incident display string used everywhere
// a title is shown in the UI (list, detail header, dashboard table,
// notifications).
export function formatIncidentTitle(incidentNumber: number, title: string): string {
  return `${formatIncidentCode(incidentNumber)} - ${title}`;
}
