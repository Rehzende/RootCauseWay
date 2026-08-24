import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { WarRoomPanel } from '@/components/WarRoomPanel';
import { ToastProvider } from '@/components/Toast';
import * as warRoomHooks from '@/hooks/useWarRoom';

vi.mock('@/hooks/useWarRoom');

function renderPanel(incidentId: string) {
  return render(
    <ToastProvider>
      <WarRoomPanel incidentId={incidentId} />
    </ToastProvider>,
  );
}

const mockUseWarRoom = vi.mocked(warRoomHooks.useWarRoom);
const mockUseStartWarRoom = vi.mocked(warRoomHooks.useStartWarRoom);
const mockUseEndWarRoom = vi.mocked(warRoomHooks.useEndWarRoom);

function mockMutation(overrides: Record<string, unknown> = {}) {
  return { mutate: vi.fn(), isPending: false, ...overrides } as any;
}

beforeEach(() => {
  vi.clearAllMocks();
  mockUseStartWarRoom.mockReturnValue(mockMutation());
  mockUseEndWarRoom.mockReturnValue(mockMutation());
});

describe('WarRoomPanel', () => {
  it('renders the start button when no meeting exists yet', () => {
    mockUseWarRoom.mockReturnValue({ data: null, isLoading: false } as any);
    renderPanel('inc-1');
    expect(screen.getByText('Start War Room')).toBeInTheDocument();
  });

  it('starts a war room when the button is clicked', async () => {
    const mutate = vi.fn();
    mockUseWarRoom.mockReturnValue({ data: null, isLoading: false } as any);
    mockUseStartWarRoom.mockReturnValue(mockMutation({ mutate }));
    renderPanel('inc-1');
    await userEvent.click(screen.getByText('Start War Room'));
    expect(mutate).toHaveBeenCalledTimes(1);
  });

  it('renders status badge and join link once a meeting exists', () => {
    mockUseWarRoom.mockReturnValue({
      data: {
        id: 'meeting-1',
        status: 'active',
        join_url: 'https://teams.microsoft.com/join/meeting-1',
      },
      isLoading: false,
    } as any);
    renderPanel('inc-1');
    expect(screen.getByText('active')).toBeInTheDocument();
    const joinLink = screen.getByText('Join Teams Meeting').closest('a');
    expect(joinLink).toHaveAttribute('href', 'https://teams.microsoft.com/join/meeting-1');
    expect(screen.getByText('End War Room')).toBeInTheDocument();
  });

  it('ends the war room when End War Room is clicked', async () => {
    const mutate = vi.fn();
    mockUseWarRoom.mockReturnValue({
      data: { id: 'meeting-1', status: 'scheduled', join_url: 'https://teams.microsoft.com/join/meeting-1' },
      isLoading: false,
    } as any);
    mockUseEndWarRoom.mockReturnValue(mockMutation({ mutate }));
    renderPanel('inc-1');
    await userEvent.click(screen.getByText('End War Room'));
    expect(mutate).toHaveBeenCalledWith('meeting-1');
  });

  it('renders executive summary, action items, and participants once summarized', () => {
    mockUseWarRoom.mockReturnValue({
      data: {
        id: 'meeting-1',
        status: 'summarized',
        join_url: 'https://teams.microsoft.com/join/meeting-1',
        summary: {
          executive_summary: 'Database failover caused elevated latency for 12 minutes.',
          key_points: ['Failover triggered at 10:02 UTC'],
          action_items: [{ description: 'Add automated failover alert', owner_hint: 'SRE team' }],
        },
        attendance: [{ name: 'Jane Doe', email: 'jane@example.com' }],
      },
      isLoading: false,
    } as any);
    renderPanel('inc-1');
    expect(screen.getByText('Database failover caused elevated latency for 12 minutes.')).toBeInTheDocument();
    expect(screen.getByText(/Add automated failover alert/)).toBeInTheDocument();
    expect(screen.getByText('Jane Doe')).toBeInTheDocument();
    // No manual end button once summarized -- meeting is over.
    expect(screen.queryByText('End War Room')).not.toBeInTheDocument();
  });
});
