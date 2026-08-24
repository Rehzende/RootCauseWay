import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ToastProvider } from '@/components/Toast';
import { SkillDetail } from '@/pages/SkillsPage';
import * as apiModule from '@/services/api';
import type { Skill } from '@/types/api';

vi.mock('@/services/api', async () => {
  const actual = await vi.importActual<typeof apiModule>('@/services/api');
  return { ...actual, updateSkill: vi.fn(), listA2AAgents: vi.fn(), listAgentSkills: vi.fn() };
});

const mockUpdateSkill = vi.mocked(apiModule.updateSkill);
const mockListA2AAgents = vi.mocked(apiModule.listA2AAgents);

function makeSkill(overrides: Partial<Skill> = {}): Skill {
  return {
    id: 'skill-1',
    org_id: 'org-1',
    name: 'Memory Leak Deep Dive',
    slug: 'memory-leak-deep-dive',
    description: 'Investigates memory growth patterns.',
    category: 'application',
    prompt_template: '',
    required_resource_types: [],
    required_permissions: [],
    enabled: true,
    created_at: '2026-08-20T00:00:00Z',
    updated_at: '2026-08-20T00:00:00Z',
    ...overrides,
  };
}

function renderDetail(skill: Skill) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <MemoryRouter>
      <QueryClientProvider client={queryClient}>
        <ToastProvider>
          <SkillDetail skill={skill} onBack={() => {}} />
        </ToastProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockListA2AAgents.mockResolvedValue({ data: [], total: 0, page: 1, per_page: 20 });
});

describe('SkillDetail', () => {
  // Regression: a user hit this live -- the enable/disable toggle used to
  // call updateSkill(id, { enabled: !skill.enabled }) with nothing else.
  // The backend 400'd every time (name/slug are binding:"required" on
  // CreateSkillRequest, which UpdateSkill also binds into), so the toggle
  // silently did nothing but show an error toast -- which a user
  // correctly read as "there's no way to edit this skill".
  it('toggling enabled resends the skill\'s full current data, not just {enabled}', async () => {
    const skill = makeSkill({ enabled: true });
    mockUpdateSkill.mockResolvedValue({ ...skill, enabled: false });

    renderDetail(skill);

    fireEvent.click(screen.getByRole('button', { name: /disable/i }));

    await waitFor(() => expect(mockUpdateSkill).toHaveBeenCalledTimes(1));
    const [id, payload] = mockUpdateSkill.mock.calls[0];
    expect(id).toBe('skill-1');
    expect(payload).toMatchObject({
      name: 'Memory Leak Deep Dive',
      slug: 'memory-leak-deep-dive',
      description: 'Investigates memory growth patterns.',
      category: 'application',
      enabled: false,
    });
  });

  it('has an Edit button that opens a form pre-filled with the skill\'s data', async () => {
    const skill = makeSkill();
    renderDetail(skill);

    fireEvent.click(screen.getByRole('button', { name: /edit/i }));

    expect(await screen.findByText('Edit Skill')).toBeInTheDocument();
    const nameInput = screen.getByDisplayValue('Memory Leak Deep Dive');
    expect(nameInput).toBeInTheDocument();
  });

  it('submitting the edit form sends the updated fields plus the skill\'s current enabled state', async () => {
    const skill = makeSkill({ enabled: true });
    mockUpdateSkill.mockResolvedValue(skill);
    renderDetail(skill);

    fireEvent.click(screen.getByRole('button', { name: /edit/i }));
    const nameInput = await screen.findByDisplayValue('Memory Leak Deep Dive');
    fireEvent.change(nameInput, { target: { value: 'Memory Leak Investigator' } });
    fireEvent.click(screen.getByRole('button', { name: /save changes/i }));

    await waitFor(() => expect(mockUpdateSkill).toHaveBeenCalledTimes(1));
    const [, payload] = mockUpdateSkill.mock.calls[0];
    expect(payload).toMatchObject({ name: 'Memory Leak Investigator', enabled: true });
  });
});
