import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AuthProvider, useAuth } from '@/hooks/useAuth';
import * as apiModule from '@/services/api';

vi.mock('@/services/api', async () => {
  const actual = await vi.importActual<typeof apiModule>('@/services/api');
  return {
    ...actual,
    login: vi.fn(),
    getCurrentUser: vi.fn().mockRejectedValue(new Error('not authenticated')),
  };
});

const mockLogin = vi.mocked(apiModule.login);
const mockGetCurrentUser = vi.mocked(apiModule.getCurrentUser);

function TestComponent() {
  const { isAuthenticated, user, login, logout } = useAuth();
  return (
    <div>
      <span data-testid="auth">{isAuthenticated ? 'yes' : 'no'}</span>
      <span data-testid="user">{user?.name ?? 'none'}</span>
      <button onClick={() => login('a@b.com', 'pass1234')}>Login</button>
      <button onClick={logout}>Logout</button>
    </div>
  );
}

beforeEach(() => {
  localStorage.clear();
  vi.clearAllMocks();
  mockGetCurrentUser.mockRejectedValue(new Error('not authenticated'));
});

describe('useAuth', () => {
  it('starts unauthenticated', () => {
    render(<AuthProvider><TestComponent /></AuthProvider>);
    expect(screen.getByTestId('auth')).toHaveTextContent('no');
  });

  it('logs in and stores token', async () => {
    const mockUser = { id: '1', org_id: 'o1', name: 'Test', email: 'a@b.com', role: 'admin', created_at: '', roles: [], permissions: {}, is_active: true } as any;
    mockLogin.mockResolvedValue({ token: 'tok123', user: mockUser });
    mockGetCurrentUser.mockResolvedValue(mockUser);

    render(<AuthProvider><TestComponent /></AuthProvider>);
    await act(async () => {
      await userEvent.click(screen.getByText('Login'));
    });

    expect(screen.getByTestId('auth')).toHaveTextContent('yes');
    expect(screen.getByTestId('user')).toHaveTextContent('Test');
    expect(localStorage.getItem('rootcauseway_token')).toBe('tok123');
  });

  it('logs out and clears token', async () => {
    localStorage.setItem('rootcauseway_token', 'tok');
    localStorage.setItem('rootcauseway_user', JSON.stringify({ name: 'X' }));

    render(<AuthProvider><TestComponent /></AuthProvider>);
    await act(async () => {
      await userEvent.click(screen.getByText('Logout'));
    });

    expect(screen.getByTestId('auth')).toHaveTextContent('no');
    expect(localStorage.getItem('rootcauseway_token')).toBeNull();
  });
});
