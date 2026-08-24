import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { SeverityBadge } from '@/components/SeverityBadge';
import type { Severity } from '@/types/api';

const severities: Severity[] = ['critical', 'high', 'medium', 'low'];

describe('SeverityBadge', () => {
  severities.forEach((severity) => {
    it(`renders ${severity} badge`, () => {
      render(<SeverityBadge severity={severity} />);
      const badge = screen.getByTestId(`severity-${severity}`);
      expect(badge).toBeInTheDocument();
      expect(badge).toHaveTextContent(severity.charAt(0).toUpperCase() + severity.slice(1));
    });
  });
});
