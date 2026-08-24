import type { SLOHealthStatus } from '@/types/api';

const styles: Record<SLOHealthStatus, string> = {
  healthy: 'bg-green-100 text-green-800 border-green-200',
  at_risk: 'bg-amber-100 text-amber-800 border-amber-200',
  exhausted: 'bg-red-100 text-red-800 border-red-200',
};

const labels: Record<SLOHealthStatus, string> = {
  healthy: 'Healthy',
  at_risk: 'At Risk',
  exhausted: 'Exhausted',
};

export function SLOStatusBadge({ status }: { status: SLOHealthStatus }) {
  return (
    <span
      data-testid={`slo-status-${status}`}
      className={`inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold ${styles[status]}`}
    >
      {labels[status]}
    </span>
  );
}
