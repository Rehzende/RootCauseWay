import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AuthProvider } from '@/hooks/useAuth';
import { SLODashboardPage } from '@/pages/SLODashboardPage';
import * as sloHooks from '@/hooks/useSLO';
import * as apiModule from '@/services/api';

vi.mock('@/hooks/useSLO');
vi.mock('@/services/api', async () => {
  const actual = await vi.importActual<typeof apiModule>('@/services/api');
  return { ...actual, listSoftware: vi.fn() };
});

const mockUseSLODefinitions = vi.mocked(sloHooks.useSLODefinitions);
const mockUseSLOStatus = vi.mocked(sloHooks.useSLOStatus);
const mockUseCreateSLODefinition = vi.mocked(sloHooks.useCreateSLODefinition);
const mockUseUpdateSLODefinition = vi.mocked(sloHooks.useUpdateSLODefinition);
const mockUseDeleteSLODefinition = vi.mocked(sloHooks.useDeleteSLODefinition);
const mockListSoftware = vi.mocked(apiModule.listSoftware);

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <SLODashboardPage />
      </AuthProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  mockListSoftware.mockResolvedValue({
    data: [{ id: 'sw-1', name: 'Checkout Service' }],
    total: 1,
    page: 1,
    per_page: 100,
  } as any);
  mockUseCreateSLODefinition.mockReturnValue({ mutate: vi.fn(), isPending: false } as any);
  mockUseUpdateSLODefinition.mockReturnValue({ mutate: vi.fn(), isPending: false } as any);
  mockUseDeleteSLODefinition.mockReturnValue({ mutate: vi.fn(), isPending: false } as any);
});

describe('SLODashboardPage', () => {
  it('renders an empty state when there are no SLO definitions', async () => {
    mockUseSLODefinitions.mockReturnValue({ data: [], isLoading: false } as any);
    renderPage();
    expect(await screen.findByText('No SLOs configured')).toBeInTheDocument();
  });

  it('renders the SLO list with correctly mapped status badges', async () => {
    mockUseSLODefinitions.mockReturnValue({
      data: [
        {
          id: 'slo-1', software_id: 'sw-1', name: 'Availability 99.9',
          slo_type: 'availability', target_percentage: 99.9, measurement_window_days: 30,
        },
        {
          id: 'slo-2', software_id: 'sw-1', name: 'Error Rate Budget',
          slo_type: 'error_rate', target_percentage: 99, measurement_window_days: 30,
        },
      ],
      isLoading: false,
    } as any);

    mockUseSLOStatus.mockImplementation((id?: string) => {
      if (id === 'slo-1') {
        return {
          data: { status: 'healthy', current_percentage: 99.95, error_budget_remaining_percentage: 80 },
          isLoading: false,
        } as any;
      }
      return {
        data: { status: 'exhausted', current_percentage: 90, error_budget_remaining_percentage: 0 },
        isLoading: false,
      } as any;
    });

    renderPage();

    expect(await screen.findByText('Availability 99.9')).toBeInTheDocument();
    expect(screen.getByText('Error Rate Budget')).toBeInTheDocument();
    expect(screen.getByTestId('slo-status-healthy')).toBeInTheDocument();
    expect(screen.getByTestId('slo-status-exhausted')).toBeInTheDocument();
    // Software name resolved from the software list
    expect(await screen.findAllByText('Checkout Service')).toHaveLength(2);
  });
});
