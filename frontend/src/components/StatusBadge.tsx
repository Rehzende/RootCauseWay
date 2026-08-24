import type { IncidentStatus } from '@/types/api';

const styles: Record<IncidentStatus, string> = {
  open: 'bg-red-100 text-red-800 border-red-200',
  investigating: 'bg-purple-100 text-purple-800 border-purple-200',
  mitigated: 'bg-yellow-100 text-yellow-800 border-yellow-200',
  resolved: 'bg-green-100 text-green-800 border-green-200',
  closed: 'bg-gray-100 text-gray-800 border-gray-200',
};

const labels: Record<IncidentStatus, string> = {
  open: 'Open',
  investigating: 'Investigating',
  mitigated: 'Mitigated',
  resolved: 'Resolved',
  closed: 'Closed',
};

export function StatusBadge({ status }: { status: IncidentStatus }) {
  return (
    <span
      data-testid={`status-${status}`}
      className={`inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold ${styles[status]}`}
    >
      {labels[status]}
    </span>
  );
}
