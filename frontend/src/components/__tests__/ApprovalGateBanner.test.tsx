import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ApprovalGateBanner } from '@/components/ApprovalGateBanner';

describe('ApprovalGateBanner', () => {
  it('renders nothing when awaiting_approval_stage is not set', () => {
    const { container } = render(<ApprovalGateBanner awaitingApprovalStage={null} onApprove={() => {}} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('shows the banner with the paused stage name when awaiting approval', () => {
    render(<ApprovalGateBanner awaitingApprovalStage="postmortem" onApprove={() => {}} />);
    expect(screen.getByTestId('approval-gate-banner')).toBeInTheDocument();
    expect(screen.getByText(/awaiting approval before postmortem/)).toBeInTheDocument();
  });

  it('fires the approve mutation when the Approve button is clicked', async () => {
    const onApprove = vi.fn();
    render(<ApprovalGateBanner awaitingApprovalStage="postmortem" onApprove={onApprove} />);
    await userEvent.click(screen.getByText('Approve'));
    expect(onApprove).toHaveBeenCalledTimes(1);
  });

  it('disables the approve button while pending', () => {
    render(<ApprovalGateBanner awaitingApprovalStage="postmortem" onApprove={() => {}} isPending />);
    expect(screen.getByText('Approving...')).toBeDisabled();
  });
});
