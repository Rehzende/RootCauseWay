import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ToastProvider } from '@/components/Toast';
import { RunbookDetailPage } from '@/pages/RunbookDetailPage';
import * as apiModule from '@/services/api';
import type { Runbook, RunbookStep } from '@/types/api';

// Guards a bug found live: the drag/reorder UI's onMove handler was a
// literal no-op comment ("reorder handled by API in real impl") -- the
// up/down buttons rendered and were clickable, but reordering a runbook's
// steps did nothing at all, on top of the backend route not existing
// either (fixed separately).
vi.mock('@/services/api', async () => {
  const actual = await vi.importActual<typeof apiModule>('@/services/api');
  return {
    ...actual,
    getRunbook: vi.fn(),
    listRunbookSteps: vi.fn(),
    listRunbookExecutions: vi.fn(),
    reorderRunbookSteps: vi.fn(),
  };
});

const mockGetRunbook = vi.mocked(apiModule.getRunbook);
const mockListRunbookSteps = vi.mocked(apiModule.listRunbookSteps);
const mockListRunbookExecutions = vi.mocked(apiModule.listRunbookExecutions);
const mockReorderRunbookSteps = vi.mocked(apiModule.reorderRunbookSteps);

function makeRunbook(): Runbook {
  return {
    id: 'rb-1', name: 'Restart flaky service', slug: 'restart-flaky-service',
    trigger_conditions: {}, auto_trigger: false, enabled: true,
  };
}

function makeSteps(): RunbookStep[] {
  return [
    { id: 's1', runbook_id: 'rb-1', step_order: 0, name: 'Check dashboard', step_type: 'manual', config: {}, timeout_seconds: 300, on_failure: 'stop' },
    { id: 's2', runbook_id: 'rb-1', step_order: 1, name: 'Restart pod', step_type: 'automated', config: {}, timeout_seconds: 300, on_failure: 'stop' },
  ];
}

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <MemoryRouter initialEntries={['/runbooks/rb-1']}>
      <QueryClientProvider client={queryClient}>
        <ToastProvider>
          <Routes>
            <Route path="/runbooks/:id" element={<RunbookDetailPage />} />
          </Routes>
        </ToastProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockGetRunbook.mockResolvedValue(makeRunbook());
  mockListRunbookSteps.mockResolvedValue(makeSteps());
  mockListRunbookExecutions.mockResolvedValue([]);
  mockReorderRunbookSteps.mockResolvedValue(undefined as any);
});

describe('RunbookDetailPage step reordering', () => {
  it('calls reorderRunbookSteps with the swapped order when moving a step down', async () => {
    renderPage();

    await waitFor(() => expect(screen.getByText('Check dashboard')).toBeInTheDocument());

    fireEvent.click(screen.getByLabelText('Move Check dashboard down'));

    await waitFor(() => {
      expect(mockReorderRunbookSteps).toHaveBeenCalledWith('rb-1', ['s2', 's1']);
    });
  });

  it('calls reorderRunbookSteps with the swapped order when moving a step up', async () => {
    renderPage();

    await waitFor(() => expect(screen.getByText('Restart pod')).toBeInTheDocument());

    fireEvent.click(screen.getByLabelText('Move Restart pod up'));

    await waitFor(() => {
      expect(mockReorderRunbookSteps).toHaveBeenCalledWith('rb-1', ['s2', 's1']);
    });
  });

  it('disables the up button on the first step and the down button on the last', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText('Check dashboard')).toBeInTheDocument());

    expect(screen.getByLabelText('Move Check dashboard up')).toBeDisabled();
    expect(screen.getByLabelText('Move Restart pod down')).toBeDisabled();
  });
});
