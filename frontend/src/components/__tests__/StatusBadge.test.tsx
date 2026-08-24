import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { StatusBadge } from '@/components/StatusBadge';
import type { IncidentStatus } from '@/types/api';

const statuses: IncidentStatus[] = ['open', 'investigating', 'mitigated', 'resolved', 'closed'];

describe('StatusBadge', () => {
  statuses.forEach((status) => {
    it(`renders ${status} badge`, () => {
      render(<StatusBadge status={status} />);
      expect(screen.getByTestId(`status-${status}`)).toBeInTheDocument();
    });
  });
});
