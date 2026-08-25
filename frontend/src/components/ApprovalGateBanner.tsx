import { ShieldAlert } from 'lucide-react';
import { PermissionButton } from '@/components/PermissionButton';

interface ApprovalGateBannerProps {
  awaitingApprovalStage?: string | null;
  onApprove: () => void;
  isPending?: boolean;
}

export function ApprovalGateBanner({ awaitingApprovalStage, onApprove, isPending }: ApprovalGateBannerProps) {
  if (!awaitingApprovalStage) return null;

  return (
    <div
      data-testid="approval-gate-banner"
      className="flex items-center justify-between gap-4 rounded-lg border border-amber-200 bg-amber-50 px-5 py-4"
    >
      <div className="flex items-center gap-3">
        <ShieldAlert className="h-5 w-5 shrink-0 text-amber-500" />
        <div>
          <p className="text-sm font-semibold text-amber-900">
            Pipeline paused &mdash; awaiting approval before {awaitingApprovalStage}
          </p>
          <p className="mt-0.5 text-xs text-amber-700">
            A human must approve this stage before the orchestrator can continue.
          </p>
        </div>
      </div>
      <PermissionButton
        resource="incidents" action="write"
        onClick={onApprove}
        disabled={isPending}
        className="shrink-0 rounded-md bg-amber-600 px-4 py-2 text-sm font-medium text-white hover:bg-amber-700 disabled:opacity-50"
      >
        {isPending ? 'Approving...' : 'Approve'}
      </PermissionButton>
    </div>
  );
}
