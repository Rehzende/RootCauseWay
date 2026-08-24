import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from 'react';
import type { UserWithRoles } from '@/types/api';
import { login as apiLogin, getCurrentUser } from '@/services/api';

interface AuthContextValue {
  user: UserWithRoles | null;
  token: string | null;
  isAuthenticated: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
  hasPermission: (resource: string, action: string) => boolean;
  hasRole: (roleSlug: string) => boolean;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem('rootcauseway_token'));
  const [user, setUser] = useState<UserWithRoles | null>(() => {
    const stored = localStorage.getItem('rootcauseway_user');
    return stored ? JSON.parse(stored) : null;
  });

  // Handle SSO callback - extract token from URL params
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const ssoToken = params.get('token');
    if (ssoToken) {
      localStorage.setItem('rootcauseway_token', ssoToken);
      setToken(ssoToken);
      window.history.replaceState({}, '', window.location.pathname);
    }
  }, []);

  // Fetch full user with roles/permissions on mount
  useEffect(() => {
    if (token && (!user?.roles || !user?.permissions)) {
      getCurrentUser()
        .then((fullUser) => {
          setUser(fullUser);
          localStorage.setItem('rootcauseway_user', JSON.stringify(fullUser));
        })
        .catch(() => {
          // Token might be invalid
        });
    }
  }, [token]); // eslint-disable-line react-hooks/exhaustive-deps

  const login = useCallback(async (email: string, password: string) => {
    const res = await apiLogin({ email, password });
    localStorage.setItem('rootcauseway_token', res.token);
    setToken(res.token);
    // Fetch full user with roles
    try {
      const fullUser = await getCurrentUser();
      setUser(fullUser);
      localStorage.setItem('rootcauseway_user', JSON.stringify(fullUser));
    } catch {
      // Fallback to basic user from login response
      const basicUser: UserWithRoles = {
        ...res.user,
        roles: [],
        permissions: {},
        is_active: true,
      };
      setUser(basicUser);
      localStorage.setItem('rootcauseway_user', JSON.stringify(basicUser));
    }
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem('rootcauseway_token');
    localStorage.removeItem('rootcauseway_user');
    setToken(null);
    setUser(null);
  }, []);

  const hasPermission = useCallback(
    (resource: string, action: string): boolean => {
      if (!user) return false;
      if (user.role === 'admin' || user.roles?.some((r) => r.slug === 'admin')) return true;
      if (!user.permissions) return false;
      const actions = user.permissions[resource];
      if (!actions) return false;
      return actions.includes(action) || actions.includes('*');
    },
    [user],
  );

  const hasRole = useCallback(
    (roleSlug: string): boolean => {
      if (!user?.roles) return false;
      return user.roles.some((r) => r.slug === roleSlug);
    },
    [user],
  );

  return (
    <AuthContext.Provider value={{ user, token, isAuthenticated: !!token, login, logout, hasPermission, hasRole }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}
