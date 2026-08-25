import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AuthProvider } from '@/hooks/useAuth';
import { ApprovalGateBanner } from '@/components/ApprovalGateBanner';

// Approve is gated behind incidents:write (PermissionButton) -- seed an
// admin user so AuthProvider's hasPermission() allows it, same as a real
// logged-in admin would see.
function renderBanner(props: Parameters<typeof ApprovalGateBanner>[0]) {
  return render(
    <AuthProvider>
      <ApprovalGateBanner {...props} />
    </AuthProvider>,
  );
}

beforeEach(() => {
  localStorage.setItem('rootcauseway_token', 'test-token');
  localStorage.setItem('rootcauseway_user', JSON.stringify({ id: 'u-1', name: 'Test Admin', email: 'admin@test.com', role: 'admin' }));
});

describe('ApprovalGateBanner', () => {
  it('renders nothing when awaiting_approval_stage is not set', () => {
    const { container } = renderBanner({ awaitingApprovalStage: null, onApprove: () => {} });
    expect(container).toBeEmptyDOMElement();
  });

  it('shows the banner with the paused stage name when awaiting approval', () => {
    renderBanner({ awaitingApprovalStage: 'postmortem', onApprove: () => {} });
    expect(screen.getByTestId('approval-gate-banner')).toBeInTheDocument();
    expect(screen.getByText(/awaiting approval before postmortem/)).toBeInTheDocument();
  });

  it('fires the approve mutation when the Approve button is clicked', async () => {
    const onApprove = vi.fn();
    renderBanner({ awaitingApprovalStage: 'postmortem', onApprove });
    await userEvent.click(screen.getByText('Approve'));
    expect(onApprove).toHaveBeenCalledTimes(1);
  });

  it('disables the approve button while pending', () => {
    renderBanner({ awaitingApprovalStage: 'postmortem', onApprove: () => {}, isPending: true });
    expect(screen.getByText('Approving...')).toBeDisabled();
  });
});
