import { useState, useEffect, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';
import { listSSOProviders, ssoLogin } from '@/services/api';
import type { SSOProvider } from '@/types/api';

const PROVIDER_ICONS: Record<string, string> = {
  google: 'G',
  github: 'GH',
  azure_ad: 'Az',
  oidc: 'ID',
};

const PROVIDER_COLORS: Record<string, string> = {
  google: 'border-gray-300 hover:bg-gray-50',
  github: 'border-gray-800 bg-gray-900 text-white hover:bg-gray-800',
  azure_ad: 'border-blue-500 hover:bg-blue-50',
  oidc: 'border-indigo-400 hover:bg-indigo-50',
};

export function LoginPage() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [ssoProviders, setSsoProviders] = useState<SSOProvider[]>([]);
  const { login } = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    listSSOProviders()
      .then((providers) => setSsoProviders(providers.filter((p) => p.enabled)))
      .catch(() => { /* no SSO available */ });
  }, []);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await login(email, password);
      navigate('/');
    } catch {
      setError('Invalid email or password');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-8 shadow-sm">
      <h2 className="mb-6 text-center text-xl font-semibold text-gray-900">Sign in to RootCauseway</h2>
      {error && (
        <div className="mb-4 rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</div>
      )}
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label htmlFor="email" className="block text-sm font-medium text-gray-700">Email</label>
          <input
            id="email"
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
        </div>
        <div>
          <label htmlFor="password" className="block text-sm font-medium text-gray-700">Password</label>
          <input
            id="password"
            type="password"
            required
            minLength={8}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
        </div>
        <button
          type="submit"
          disabled={loading}
          className="w-full rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
        >
          {loading ? 'Signing in...' : 'Sign in'}
        </button>
      </form>

      {ssoProviders.length > 0 && (
        <>
          <div className="my-6 flex items-center gap-3">
            <div className="h-px flex-1 bg-gray-200" />
            <span className="text-xs font-medium text-gray-400">Or sign in with</span>
            <div className="h-px flex-1 bg-gray-200" />
          </div>
          <div className="space-y-2">
            {ssoProviders.map((provider) => (
              <button
                key={provider.id}
                onClick={() => ssoLogin(provider.provider_type)}
                className={`flex w-full items-center gap-3 rounded-md border px-4 py-2.5 text-sm font-medium transition-colors ${PROVIDER_COLORS[provider.provider_type] ?? 'border-gray-300 hover:bg-gray-50'}`}
              >
                <span className="flex h-5 w-5 items-center justify-center rounded text-xs font-bold">
                  {PROVIDER_ICONS[provider.provider_type] ?? 'SSO'}
                </span>
                <span>{provider.name}</span>
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
