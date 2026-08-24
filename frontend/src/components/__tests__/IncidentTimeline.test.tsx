import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { IncidentTimeline } from '@/components/IncidentTimeline';
import type { IncidentEvent } from '@/types/api';

const mockEvents: IncidentEvent[] = [
  {
    id: '1',
    incident_id: 'inc1',
    type: 'alert_received',
    actor: 'system',
    data: {},
    created_at: '2024-01-01T00:00:00Z',
  },
  {
    id: '2',
    incident_id: 'inc1',
    type: 'triage_started',
    actor: 'agent-triage',
    data: {},
    created_at: '2024-01-01T00:01:00Z',
  },
  {
    id: '3',
    incident_id: 'inc1',
    type: 'comment',
    actor: 'john',
    data: { message: 'Investigating now' },
    created_at: '2024-01-01T00:05:00Z',
  },
];

describe('IncidentTimeline', () => {
  it('renders all events', () => {
    render(<IncidentTimeline events={mockEvents} />);
    expect(screen.getByTestId('incident-timeline')).toBeInTheDocument();
    expect(screen.getByTestId('timeline-event-alert_received')).toBeInTheDocument();
    expect(screen.getByTestId('timeline-event-triage_started')).toBeInTheDocument();
    expect(screen.getByTestId('timeline-event-comment')).toBeInTheDocument();
  });

  it('shows actor names', () => {
    render(<IncidentTimeline events={mockEvents} />);
    expect(screen.getByText('by system')).toBeInTheDocument();
    expect(screen.getByText('by agent-triage')).toBeInTheDocument();
    expect(screen.getByText('by john')).toBeInTheDocument();
  });

  it('shows empty state when no events', () => {
    render(<IncidentTimeline events={[]} />);
    expect(screen.getByText('No events yet.')).toBeInTheDocument();
  });
});
