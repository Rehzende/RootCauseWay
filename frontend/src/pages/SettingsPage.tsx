import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useAuth } from '@/hooks/useAuth';
import {
  listSSOProviders, createSSOProvider, updateSSOProvider, deleteSSOProvider,
  listAPIKeys, createAPIKey, deleteAPIKey, listRoles,
  getOrganizationSettings, updateOrganizationSettings, initiateTeamsOAuth,
} from '@/services/api';
import { PermissionGate } from '@/components/PermissionGate';
import { useToast } from '@/components/Toast';
import { X, Plus, Copy, Trash2, Check } from 'lucide-react';
import type { SSOProvider, APIKeyWithSecret, CreateSSOProviderRequest, RoleWithPermissions } from '@/types/api';

type Tab = 'general' | 'sso' | 'api-keys' | 'llm' | 'integrations';

const LLM_PROVIDER_PRESETS: { value: string; label: string; baseUrlHint: string }[] = [
  { value: 'lm_studio', label: 'LM Studio (self-hosted)', baseUrlHint: 'http://lm-studio:1234/v1' },
  { value: 'openrouter', label: 'OpenRouter', baseUrlHint: 'https://openrouter.ai/api/v1' },
  { value: 'openai', label: 'OpenAI', baseUrlHint: 'https://api.openai.com/v1' },
  { value: 'anthropic', label: 'Anthropic', baseUrlHint: 'https://api.anthropic.com/v1' },
  { value: 'custom', label: 'Custom / other', baseUrlHint: 'https://your-provider.example/v1' },
];

const PROVIDER_TYPE_ICONS: Record<string, string> = {
  google: 'G',
  github: 'GH',
  azure_ad: 'Az',
  oidc: 'ID',
};

// --- SSO Provider Form Modal ---
function SSOProviderModal({ provider, onClose }: { provider?: SSOProvider; onClose: () => void }) {
  const queryClient = useQueryClient();
  const [form, setForm] = useState<CreateSSOProviderRequest>({
    name: provider?.name ?? '',
    provider_type: provider?.provider_type ?? 'oidc',
    client_id: provider?.client_id ?? '',
    client_secret: '',
    issuer_url: provider?.issuer_url ?? '',
    auto_provision_users: provider?.auto_provision_users ?? false,
  });

  const createMut = useMutation({
    mutationFn: () => createSSOProvider(form),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['sso-providers'] }); onClose(); },
  });

  const updateMut = useMutation({
    mutationFn: () => updateSSOProvider(provider!.id, { ...form, enabled: provider?.enabled }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['sso-providers'] }); onClose(); },
  });

  const isEdit = !!provider;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-md rounded-lg bg-white p-6 shadow-xl">
        <div className="mb-4 flex items-center justify-between">
          <h3 className="text-lg font-semibold text-gray-900">{isEdit ? 'Edit' : 'Add'} SSO Provider</h3>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
        </div>
        <form onSubmit={(e) => { e.preventDefault(); isEdit ? updateMut.mutate() : createMut.mutate(); }} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700">Name</label>
            <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Provider Type</label>
            <select value={form.provider_type} onChange={(e) => setForm({ ...form, provider_type: e.target.value })} className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm">
              <option value="google">Google</option>
              <option value="github">GitHub</option>
              <option value="azure_ad">Azure AD</option>
              <option value="oidc">OIDC</option>
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Client ID</label>
            <input value={form.client_id} onChange={(e) => setForm({ ...form, client_id: e.target.value })} required className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Client Secret</label>
            <input type="password" value={form.client_secret} onChange={(e) => setForm({ ...form, client_secret: e.target.value })} required={!isEdit} placeholder={isEdit ? '(unchanged)' : ''} className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Issuer URL</label>
            <input value={form.issuer_url ?? ''} onChange={(e) => setForm({ ...form, issuer_url: e.target.value })} className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
          </div>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={form.auto_provision_users} onChange={(e) => setForm({ ...form, auto_provision_users: e.target.checked })} className="rounded border-gray-300" />
            Auto-provision users
          </label>
          <div className="flex justify-end gap-2">
            <button type="button" onClick={onClose} className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
            <button type="submit" disabled={createMut.isPending || updateMut.isPending} className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">
              {(createMut.isPending || updateMut.isPending) ? 'Saving...' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// --- API Key Created Modal ---
function APIKeyCreatedModal({ apiKey, onClose }: { apiKey: APIKeyWithSecret; onClose: () => void }) {
  const [copied, setCopied] = useState(false);

  const copyKey = () => {
    navigator.clipboard.writeText(apiKey.key);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-md rounded-lg bg-white p-6 shadow-xl">
        <h3 className="text-lg font-semibold text-gray-900">API Key Created</h3>
        <p className="mt-2 text-sm text-amber-600">Copy this key now. You will not be able to see it again.</p>
        <div className="mt-4 flex items-center gap-2 rounded-md bg-gray-900 px-4 py-3">
          <code className="flex-1 break-all font-mono text-sm text-green-400">{apiKey.key}</code>
          <button onClick={copyKey} className="text-gray-400 hover:text-white">
            {copied ? <Check className="h-4 w-4 text-green-400" /> : <Copy className="h-4 w-4" />}
          </button>
        </div>
        <div className="mt-4 flex justify-end">
          <button onClick={onClose} className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700">Done</button>
        </div>
      </div>
    </div>
  );
}

// --- Create API Key Modal ---
function CreateAPIKeyModal({ onCreated, onClose }: { onCreated: (key: APIKeyWithSecret) => void; onClose: () => void }) {
  const [name, setName] = useState('');
  const [selectedRoleId, setSelectedRoleId] = useState('');
  const [expiresAt, setExpiresAt] = useState('');

  const { data: rolesData } = useQuery({ queryKey: ['roles'], queryFn: () => listRoles({ per_page: 100 }) });
  const roles: RoleWithPermissions[] = rolesData?.data ?? [];

  const mutation = useMutation({
    mutationFn: () => createAPIKey({ name, role_id: selectedRoleId || undefined, scopes: ['read'], expires_at: expiresAt || undefined }),
    onSuccess: (key) => { onCreated(key); },
  });

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-md rounded-lg bg-white p-6 shadow-xl">
        <div className="mb-4 flex items-center justify-between">
          <h3 className="text-lg font-semibold text-gray-900">Create API Key</h3>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
        </div>
        <form onSubmit={(e) => { e.preventDefault(); mutation.mutate(); }} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700">Name</label>
            <input value={name} onChange={(e) => setName(e.target.value)} required className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Role</label>
            <select value={selectedRoleId} onChange={(e) => setSelectedRoleId(e.target.value)} required className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm">
              <option value="">Select a role...</option>
              {roles.map((role) => (
                <option key={role.id} value={role.id}>{role.name}</option>
              ))}
            </select>
            <p className="mt-1 text-xs text-gray-500">The selected role determines the API key's permissions</p>
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Expires At (optional)</label>
            <input type="datetime-local" value={expiresAt} onChange={(e) => setExpiresAt(e.target.value)} className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
          </div>
          <div className="flex justify-end gap-2">
            <button type="button" onClick={onClose} className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
            <button type="submit" disabled={mutation.isPending} className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">Create</button>
          </div>
        </form>
      </div>
    </div>
  );
}

export function SettingsPage() {
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [tab, setTab] = useState<Tab>('general');
  const [showSSOModal, setShowSSOModal] = useState(false);
  const [editSSO, setEditSSO] = useState<SSOProvider | undefined>();
  const [showCreateKey, setShowCreateKey] = useState(false);
  const [createdKey, setCreatedKey] = useState<APIKeyWithSecret | null>(null);
  const [hitlGateEnabled, setHitlGateEnabled] = useState(false);
  const [llmProviderType, setLlmProviderType] = useState('lm_studio');
  const [llmBaseUrl, setLlmBaseUrl] = useState('');
  const [llmModel, setLlmModel] = useState('');
  const [llmApiKeyRef, setLlmApiKeyRef] = useState('');
  const [teamsTenantId, setTeamsTenantId] = useState('');
  const [teamsClientId, setTeamsClientId] = useState('');
  const [teamsClientSecret, setTeamsClientSecret] = useState('');

  const { data: orgSettings } = useQuery({
    queryKey: ['org-settings', user?.org_id],
    queryFn: () => getOrganizationSettings(user!.org_id),
    enabled: (tab === 'general' || tab === 'llm' || tab === 'integrations') && !!user?.org_id,
  });

  useEffect(() => {
    if (orgSettings) setHitlGateEnabled(orgSettings.pipeline_hitl_gate_enabled);
  }, [orgSettings]);

  useEffect(() => {
    if (!orgSettings) return;
    setLlmProviderType(orgSettings.default_llm_provider_type || 'lm_studio');
    setLlmBaseUrl(orgSettings.default_llm_base_url || '');
    setLlmModel(orgSettings.default_llm_model || '');
    setLlmApiKeyRef(orgSettings.default_llm_api_key_ref || '');
  }, [orgSettings]);

  useEffect(() => {
    if (!orgSettings) return;
    setTeamsTenantId(orgSettings.teams_tenant_id || '');
    setTeamsClientId(orgSettings.teams_client_id || '');
    // Client secret is never returned by the API -- leave the field blank
    // (the placeholder communicates whether one is already set).
    // teams_connected_account is read-only, shown straight from
    // orgSettings below -- no local state needed for it.
  }, [orgSettings]);

  // Teams OAuth connect flow lands back here via a full-page redirect (see
  // TeamsOAuthHandler.Callback) with ?teams_connected=true or
  // ?teams_error=... -- same read-once-and-strip idiom useAuth.tsx already
  // uses for the SSO login callback's ?token=.
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const connected = params.get('teams_connected');
    const error = params.get('teams_error');
    if (!connected && !error) return;

    setTab('integrations');
    if (connected) {
      queryClient.invalidateQueries({ queryKey: ['org-settings', user?.org_id] });
      addToast({ type: 'success', title: 'Teams account connected' });
    } else if (error) {
      addToast({ type: 'error', title: 'Failed to connect Teams account', message: error });
    }
    window.history.replaceState({}, '', window.location.pathname);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const hitlGateMut = useMutation({
    mutationFn: (enabled: boolean) => updateOrganizationSettings(user!.org_id, { pipeline_hitl_gate_enabled: enabled }),
    onSuccess: (data) => {
      setHitlGateEnabled(data.pipeline_hitl_gate_enabled);
      addToast({ type: 'success', title: `HITL approval gate ${data.pipeline_hitl_gate_enabled ? 'enabled' : 'disabled'}` });
    },
    onError: (err: any) => {
      addToast({ type: 'error', title: 'Failed to update pipeline settings', message: err?.response?.data?.error || err.message });
    },
  });

  const llmSettingsMut = useMutation({
    mutationFn: () => updateOrganizationSettings(user!.org_id, {
      default_llm_provider_type: llmProviderType,
      default_llm_base_url: llmBaseUrl,
      default_llm_model: llmModel,
      default_llm_api_key_ref: llmApiKeyRef,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['org-settings', user?.org_id] });
      addToast({ type: 'success', title: 'Default LLM provider saved' });
    },
    onError: (err: any) => {
      addToast({ type: 'error', title: 'Failed to save LLM settings', message: err?.response?.data?.error || err.message });
    },
  });

  const teamsSettingsMut = useMutation({
    mutationFn: () => updateOrganizationSettings(user!.org_id, {
      teams_tenant_id: teamsTenantId,
      teams_client_id: teamsClientId,
      // Omit entirely (not blank) when unchanged, so a save that only
      // touches tenant/client doesn't wipe out an already-set secret --
      // backend partial-update semantics treat a missing field as "leave
      // unchanged", an empty string as "set to empty".
      ...(teamsClientSecret ? { teams_client_secret: teamsClientSecret } : {}),
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['org-settings', user?.org_id] });
      setTeamsClientSecret('');
      addToast({ type: 'success', title: 'Teams integration saved' });
    },
    onError: (err: any) => {
      addToast({ type: 'error', title: 'Failed to save Teams integration', message: err?.response?.data?.error || err.message });
    },
  });

  // Connect a service/bot Microsoft account via delegated OAuth -- replaces
  // the old app-only auth flow, which needed a tenant admin to grant a
  // Microsoft Application Access Policy via PowerShell. Full-page nav (like
  // ssoLogin), since the browser has to land on Microsoft's own consent
  // screen; there's nothing to await client-side beyond getting the URL.
  const connectTeamsMut = useMutation({
    mutationFn: () => initiateTeamsOAuth(user!.org_id),
    onSuccess: (data) => {
      window.location.href = data.authorize_url;
    },
    onError: (err: any) => {
      addToast({ type: 'error', title: 'Failed to start Teams connection', message: err?.response?.data?.error || err.message });
    },
  });

  const { data: ssoProviders } = useQuery({ queryKey: ['sso-providers'], queryFn: listSSOProviders, enabled: tab === 'sso' });
  const { data: apiKeys } = useQuery({ queryKey: ['api-keys'], queryFn: listAPIKeys, enabled: tab === 'api-keys' });

  const toggleSSOEnabled = useMutation({
    mutationFn: (p: SSOProvider) => updateSSOProvider(p.id, { enabled: !p.enabled }),
    onSuccess: (_, p) => { queryClient.invalidateQueries({ queryKey: ['sso-providers'] }); addToast({ type: 'success', title: `SSO provider ${p.enabled ? 'disabled' : 'enabled'}` }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to update SSO provider', message: err?.response?.data?.error || err.message }); },
  });

  const deleteSSOProviderMut = useMutation({
    mutationFn: deleteSSOProvider,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['sso-providers'] }); addToast({ type: 'success', title: 'SSO provider deleted' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to delete SSO provider', message: err?.response?.data?.error || err.message }); },
  });

  const deleteAPIKeyMut = useMutation({
    mutationFn: deleteAPIKey,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['api-keys'] }); addToast({ type: 'success', title: 'API key revoked' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to revoke API key', message: err?.response?.data?.error || err.message }); },
  });

  const tabs: { key: Tab; label: string }[] = [
    { key: 'general', label: 'General' },
    { key: 'sso', label: 'SSO Providers' },
    { key: 'api-keys', label: 'API Keys' },
    { key: 'llm', label: 'LLM & Tokens' },
    { key: 'integrations', label: 'Integrations' },
  ];

  return (
    <div className="p-8">
      <h1 className="text-2xl font-bold text-gray-900">Settings</h1>
      <p className="mt-1 text-sm text-gray-500">Manage your account, SSO providers, and API keys</p>

      {/* Tabs */}
      <div className="mt-6 border-b border-gray-200">
        <div className="flex gap-6">
          {tabs.map((t) => (
            <button
              key={t.key}
              onClick={() => setTab(t.key)}
              className={`border-b-2 pb-3 text-sm font-medium transition-colors ${tab === t.key ? 'border-blue-500 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-700'}`}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>

      <div className="mt-6 max-w-4xl">
        {/* General Tab */}
        {tab === 'general' && (
          <div className="space-y-6">
            <div className="rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
              <h3 className="text-sm font-medium text-gray-900">Profile</h3>
              <div className="mt-4 space-y-3">
                <div>
                  <label className="block text-xs font-medium text-gray-500">Name</label>
                  <p className="mt-1 text-sm text-gray-900">{user?.name ?? '-'}</p>
                </div>
                <div>
                  <label className="block text-xs font-medium text-gray-500">Email</label>
                  <p className="mt-1 text-sm text-gray-900">{user?.email ?? '-'}</p>
                </div>
                <div>
                  <label className="block text-xs font-medium text-gray-500">Roles</label>
                  <div className="mt-1 flex flex-wrap gap-1">
                    {user?.roles?.map((r) => (
                      <span key={r.id} className="rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700">{r.name}</span>
                    )) ?? <span className="text-sm text-gray-400">-</span>}
                  </div>
                </div>
              </div>
            </div>
            <div className="rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
              <h3 className="text-sm font-medium text-gray-900">API</h3>
              <p className="mt-2 text-sm text-gray-500">
                API base URL: <code className="rounded bg-gray-100 px-1.5 py-0.5 text-xs">/api/v1</code>
              </p>
            </div>
            <div className="rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
              <h3 className="text-sm font-medium text-gray-900">Pipeline</h3>
              <div className="mt-4 flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium text-gray-700">HITL approval gate</p>
                  <p className="mt-0.5 text-xs text-gray-500">
                    When enabled, the orchestrator pauses before generating the postmortem and
                    waits for a human to approve via the incident's approval banner.
                  </p>
                </div>
                <PermissionGate resource="settings" action="write">
                  <label className="relative inline-flex cursor-pointer items-center">
                    <input
                      type="checkbox"
                      checked={hitlGateEnabled}
                      disabled={hitlGateMut.isPending}
                      onChange={(e) => hitlGateMut.mutate(e.target.checked)}
                      className="peer sr-only"
                    />
                    <div className="peer h-6 w-11 rounded-full bg-gray-200 after:absolute after:left-[2px] after:top-[2px] after:h-5 after:w-5 after:rounded-full after:bg-white after:transition-all after:content-[''] peer-checked:bg-blue-600 peer-checked:after:translate-x-full peer-focus:outline-none" />
                  </label>
                </PermissionGate>
              </div>
            </div>
          </div>
        )}

        {/* SSO Tab */}
        {tab === 'sso' && (
          <div>
            <div className="mb-4 flex justify-end">
              <PermissionGate resource="sso" action="write">
                <button onClick={() => { setEditSSO(undefined); setShowSSOModal(true); }} className="flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700">
                  <Plus className="h-4 w-4" /> Add Provider
                </button>
              </PermissionGate>
            </div>
            <div className="space-y-3">
              {(ssoProviders ?? []).length === 0 && (
                <p className="text-sm text-gray-400">No SSO providers configured.</p>
              )}
              {(ssoProviders ?? []).map((p) => (
                <div key={p.id} className="flex items-center justify-between rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
                  <div className="flex items-center gap-3">
                    <span className="flex h-10 w-10 items-center justify-center rounded-lg bg-gray-100 text-sm font-bold text-gray-600">
                      {PROVIDER_TYPE_ICONS[p.provider_type] ?? 'SSO'}
                    </span>
                    <div>
                      <p className="text-sm font-medium text-gray-900">{p.name}</p>
                      <p className="text-xs text-gray-400">
                        {p.provider_type} | Client ID: {p.client_id.slice(0, 8)}...
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-3">
                    <label className="flex items-center gap-2 text-sm">
                      <input
                        type="checkbox"
                        checked={p.enabled}
                        onChange={() => toggleSSOEnabled.mutate(p)}
                        className="rounded border-gray-300"
                      />
                      Enabled
                    </label>
                    <PermissionGate resource="sso" action="write">
                      <button onClick={() => { setEditSSO(p); setShowSSOModal(true); }} className="text-sm text-blue-600 hover:text-blue-800">Edit</button>
                    </PermissionGate>
                    <PermissionGate resource="sso" action="delete">
                      <button onClick={() => { if (confirm('Delete this SSO provider?')) deleteSSOProviderMut.mutate(p.id); }} className="text-gray-400 hover:text-red-600">
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </PermissionGate>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* API Keys Tab */}
        {tab === 'api-keys' && (
          <div>
            <div className="mb-4 flex justify-end">
              <PermissionGate resource="settings" action="write">
                <button onClick={() => setShowCreateKey(true)} className="flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700">
                  <Plus className="h-4 w-4" /> Create API Key
                </button>
              </PermissionGate>
            </div>
            <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white shadow-sm">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Name</th>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Key Prefix</th>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Created</th>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Last Used</th>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Expires</th>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Status</th>
                    <th className="px-6 py-3" />
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200">
                  {(apiKeys ?? []).map((key) => (
                    <tr key={key.id}>
                      <td className="whitespace-nowrap px-6 py-4 text-sm font-medium text-gray-900">{key.name}</td>
                      <td className="whitespace-nowrap px-6 py-4 font-mono text-sm text-gray-500">{key.key_prefix}...</td>
                      <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">{new Date(key.created_at).toLocaleDateString()}</td>
                      <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">{key.last_used_at ? new Date(key.last_used_at).toLocaleDateString() : 'Never'}</td>
                      <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">{key.expires_at ? new Date(key.expires_at).toLocaleDateString() : 'Never'}</td>
                      <td className="whitespace-nowrap px-6 py-4">
                        <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${key.is_active ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'}`}>
                          {key.is_active ? 'Active' : 'Revoked'}
                        </span>
                      </td>
                      <td className="whitespace-nowrap px-6 py-4 text-right">
                        <PermissionGate resource="settings" action="delete">
                          <button onClick={() => { if (confirm('Revoke this API key?')) deleteAPIKeyMut.mutate(key.id); }} className="text-sm text-red-600 hover:text-red-800">Revoke</button>
                        </PermissionGate>
                      </td>
                    </tr>
                  ))}
                  {(apiKeys ?? []).length === 0 && (
                    <tr><td colSpan={7} className="px-6 py-8 text-center text-sm text-gray-400">No API keys created yet</td></tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {/* LLM & Tokens Tab */}
        {tab === 'llm' && (
          <div className="space-y-6">
            <div className="rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
              <h3 className="text-sm font-medium text-gray-900">Default LLM Provider</h3>
              <p className="mt-1 text-xs text-gray-500">
                Applies to every managed agent (triage, evidence, RCA, postmortem) unless a
                specific agent overrides the model on the{' '}
                <a href="/agents" className="text-blue-600 hover:underline">Agents</a> page.
                BYOA agents are unaffected -- they always use their own credentials.
              </p>
              <form
                onSubmit={(e) => { e.preventDefault(); llmSettingsMut.mutate(); }}
                className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2"
              >
                <div>
                  <label className="block text-sm font-medium text-gray-700">Provider</label>
                  <select
                    value={llmProviderType}
                    onChange={(e) => setLlmProviderType(e.target.value)}
                    className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
                  >
                    {LLM_PROVIDER_PRESETS.map((p) => (
                      <option key={p.value} value={p.value}>{p.label}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700">Model</label>
                  <input
                    value={llmModel}
                    onChange={(e) => setLlmModel(e.target.value)}
                    placeholder="e.g. qwen2.5-coder-14b-instruct"
                    className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
                  />
                </div>
                <div className="sm:col-span-2">
                  <label className="block text-sm font-medium text-gray-700">Base URL</label>
                  <input
                    value={llmBaseUrl}
                    onChange={(e) => setLlmBaseUrl(e.target.value)}
                    placeholder={LLM_PROVIDER_PRESETS.find((p) => p.value === llmProviderType)?.baseUrlHint}
                    className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
                  />
                </div>
                <div className="sm:col-span-2">
                  <label className="block text-sm font-medium text-gray-700">API Key</label>
                  <input
                    type="password"
                    value={llmApiKeyRef}
                    onChange={(e) => setLlmApiKeyRef(e.target.value)}
                    placeholder="Leave blank to keep using the platform's own key"
                    className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
                  />
                  <p className="mt-1 text-xs text-gray-500">
                    Only needed when Provider is not the platform default -- e.g. pasting an
                    OpenRouter or OpenAI key to route managed agents through that provider instead.
                  </p>
                </div>
                <div className="sm:col-span-2 flex justify-end">
                  <PermissionGate resource="settings" action="write">
                    <button
                      type="submit"
                      disabled={llmSettingsMut.isPending}
                      className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
                    >
                      {llmSettingsMut.isPending ? 'Saving...' : 'Save'}
                    </button>
                  </PermissionGate>
                </div>
              </form>
            </div>
            <div className="rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
              <h3 className="text-sm font-medium text-gray-900">Token Usage &amp; Cost</h3>
              <p className="mt-1 text-xs text-gray-500">
                Real per-run token counts and cost by model/incident are tracked on the{' '}
                <a href="/analytics" className="text-blue-600 hover:underline">Analytics</a> page's
                Cost Analysis section.
              </p>
            </div>
          </div>
        )}

        {/* Integrations Tab */}
        {tab === 'integrations' && (
          <div className="space-y-6">
            <div className="rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-medium text-gray-900">Microsoft Teams (War Room)</h3>
                <span
                  className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${
                    orgSettings?.teams_configured
                      ? 'bg-green-100 text-green-800'
                      : 'bg-gray-100 text-gray-600'
                  }`}
                >
                  {orgSettings?.teams_configured ? 'Configured' : 'Not configured'}
                </span>
              </div>
              <p className="mt-1 text-xs text-gray-500">
                Lets incidents create real Teams meetings and fetch their transcript/attendance
                when a War Room is ended. A service/bot Microsoft account connects once below --
                no tenant-admin PowerShell step needed, unlike the old app-only setup.
              </p>
              <form
                onSubmit={(e) => { e.preventDefault(); teamsSettingsMut.mutate(); }}
                className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2"
              >
                <div>
                  <label className="block text-sm font-medium text-gray-700">Tenant ID</label>
                  <input
                    value={teamsTenantId}
                    onChange={(e) => setTeamsTenantId(e.target.value)}
                    placeholder="Azure AD tenant (directory) ID"
                    className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700">Client ID</label>
                  <input
                    value={teamsClientId}
                    onChange={(e) => setTeamsClientId(e.target.value)}
                    placeholder="App registration's application (client) ID"
                    className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700">Client Secret</label>
                  <input
                    type="password"
                    value={teamsClientSecret}
                    onChange={(e) => setTeamsClientSecret(e.target.value)}
                    placeholder={orgSettings?.teams_client_secret_set ? 'Already set -- leave blank to keep it' : 'App registration client secret value'}
                    className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
                  />
                </div>
                <div className="sm:col-span-2 flex justify-end">
                  <PermissionGate resource="settings" action="write">
                    <button
                      type="submit"
                      disabled={teamsSettingsMut.isPending}
                      className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
                    >
                      {teamsSettingsMut.isPending ? 'Saving...' : 'Save'}
                    </button>
                  </PermissionGate>
                </div>
              </form>

              <div className="mt-6 border-t border-gray-200 pt-6">
                <label className="block text-sm font-medium text-gray-700">Connected account</label>
                <p className="mt-1 text-sm text-gray-900">
                  {orgSettings?.teams_connected_account || 'Not connected'}
                </p>
                <p className="mt-1 text-xs text-gray-500">
                  Meetings are created as this account. Connect a dedicated service/bot Microsoft
                  account here -- it needs a Teams-capable license, but not tenant-admin rights.
                </p>
                <PermissionGate resource="settings" action="write">
                  <button
                    type="button"
                    onClick={() => connectTeamsMut.mutate()}
                    disabled={!orgSettings?.teams_client_secret_set || connectTeamsMut.isPending}
                    title={!orgSettings?.teams_client_secret_set ? 'Save Tenant ID, Client ID and Client Secret first' : undefined}
                    className="mt-3 rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {connectTeamsMut.isPending
                      ? 'Connecting...'
                      : orgSettings?.teams_configured ? 'Reconnect Teams account' : 'Connect Teams account'}
                  </button>
                </PermissionGate>
              </div>
            </div>
          </div>
        )}
      </div>

      {showSSOModal && <SSOProviderModal provider={editSSO} onClose={() => setShowSSOModal(false)} />}
      {showCreateKey && (
        <CreateAPIKeyModal
          onCreated={(key) => { setCreatedKey(key); setShowCreateKey(false); queryClient.invalidateQueries({ queryKey: ['api-keys'] }); addToast({ type: 'success', title: 'API key created' }); }}
          onClose={() => setShowCreateKey(false)}
        />
      )}
      {createdKey && <APIKeyCreatedModal apiKey={createdKey} onClose={() => setCreatedKey(null)} />}
    </div>
  );
}
