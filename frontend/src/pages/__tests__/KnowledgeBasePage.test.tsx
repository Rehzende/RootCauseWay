import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { KnowledgeBasePage } from '@/pages/KnowledgeBasePage';
import * as apiModule from '@/services/api';

vi.mock('@/services/api', async () => {
  const actual = await vi.importActual<typeof apiModule>('@/services/api');
  return { ...actual, searchKnowledgeBase: vi.fn() };
});

const mockSearchKnowledgeBase = vi.mocked(apiModule.searchKnowledgeBase);

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <KnowledgeBasePage />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe('KnowledgeBasePage', () => {
  // Regression: the backend returns a bare array from /knowledge-base/search
  // (never wrapped in {data, total, ...}), but the page used to read
  // `data?.data`, which is always undefined on an array -- so every entry
  // the API ever returned was silently discarded and the page showed the
  // empty state no matter what.
  it('renders entries from the bare array the API returns (not data.data)', async () => {
    mockSearchKnowledgeBase.mockResolvedValue([
      {
        id: 'kb-1',
        org_id: 'org-1',
        incident_id: null,
        software_id: null,
        category: 'infrastructure',
        error_pattern: 'ConnectionPoolExhausted',
        root_cause_summary: 'Pool size too small for peak load',
        resolution_summary: 'Increased max_connections',
        lessons_learned: ['Load test before release'],
        action_items: [],
        tags: [],
        human_validated: true,
        confidence: 0.9,
        times_referenced: 3,
        created_at: '2026-08-20T00:00:00Z',
        updated_at: '2026-08-20T00:00:00Z',
      },
    ] as any);

    renderPage();

    expect(await screen.findByText('Pool size too small for peak load')).toBeInTheDocument();
    expect(screen.getByText('Increased max_connections')).toBeInTheDocument();
    expect(screen.queryByText('No knowledge base entries found')).not.toBeInTheDocument();
  });

  it('shows the empty state when the API returns no entries', async () => {
    mockSearchKnowledgeBase.mockResolvedValue([]);
    renderPage();
    expect(await screen.findByText('No knowledge base entries found')).toBeInTheDocument();
  });
});
