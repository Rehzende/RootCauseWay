import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { EvidencePanel } from '@/components/EvidencePanel';
import type { IncidentEvidence } from '@/types/api';

function makeEvidence(overrides: Partial<IncidentEvidence>): IncidentEvidence {
  return {
    id: '1',
    incident_id: 'inc-1',
    type: 'trace',
    title: 'evidence',
    content: {},
    source: '',
    collected_at: '2026-08-20T00:00:00Z',
    ...overrides,
  };
}

describe('EvidencePanel', () => {
  it('shows a placeholder when there is no evidence', () => {
    render(<EvidencePanel evidence={[]} />);
    expect(screen.getByText(/no evidence collected yet/i)).toBeInTheDocument();
  });

  it('renders raw JSON content for a regular evidence entry', () => {
    const evidence = [makeEvidence({ title: 'triage result', content: { severity: 'high' } })];
    render(<EvidencePanel evidence={evidence} />);
    expect(screen.getByText('triage result')).toBeInTheDocument();
    expect(screen.getByText(/"severity": "high"/)).toBeInTheDocument();
  });

  // Platform audit backlog item: MLflow traces were persisted as evidence
  // but the frontend had no idea what to do with them -- just dumped the
  // raw {trace_id, url} JSON like any other entry, no clickable link.
  it('renders a clickable "View trace" link for MLflow trace evidence instead of raw JSON', () => {
    const evidence = [
      makeEvidence({
        title: 'MLflow pipeline trace',
        source: 'mlflow',
        content: { trace_id: 'tr-abc123', url: 'https://mlflow.rezende.lab/#/experiments/1/traces' },
      }),
    ];
    render(<EvidencePanel evidence={evidence} />);

    const link = screen.getByRole('link', { name: /view trace in mlflow/i });
    expect(link).toHaveAttribute('href', 'https://mlflow.rezende.lab/#/experiments/1/traces');
    expect(link).toHaveAttribute('target', '_blank');
    expect(screen.queryByText(/tr-abc123/)).not.toBeInTheDocument();
  });
});
