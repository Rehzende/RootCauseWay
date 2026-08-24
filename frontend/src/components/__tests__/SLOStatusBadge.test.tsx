import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { SLOStatusBadge } from '@/components/SLOStatusBadge';
import type { SLOHealthStatus } from '@/types/api';

const statuses: SLOHealthStatus[] = ['healthy', 'at_risk', 'exhausted'];
const expectedLabel: Record<SLOHealthStatus, string> = {
  healthy: 'Healthy',
  at_risk: 'At Risk',
  exhausted: 'Exhausted',
};

describe('SLOStatusBadge', () => {
  statuses.forEach((status) => {
    it(`renders the ${status} badge with the correct label`, () => {
      render(<SLOStatusBadge status={status} />);
      const badge = screen.getByTestId(`slo-status-${status}`);
      expect(badge).toBeInTheDocument();
      expect(badge).toHaveTextContent(expectedLabel[status]);
    });
  });
});
