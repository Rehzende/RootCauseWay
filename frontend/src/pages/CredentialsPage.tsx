import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Plus, Trash2, X, Shield, Clock, KeyRound } from 'lucide-react';
import { useToast } from '@/components/Toast';
import {
  listCredentialProviders, createCredentialProvider, deleteCredentialProvider,
  listAccessPolicies, createAccessPolicy, deleteAccessPolicy,
  listCredentialLeases, revokeCredentialLease,
  listSkills, listA2AAgents,
} from '@/services/api';
import type {
  CreateCredentialProviderRequest, CredentialProviderType,
  CreateAccessPolicyRequest, AccessPolicyTargetType,
  CredentialLeaseStatus,
} from '@/types/api';

const providerTypes: CredentialProviderType[] = ['hashicorp_vault', 'aws_sts', 'azure_managed_identity', 'azure_key_vault', 'gcp_workload_identity', 'static', 'custom'];
const targetTypes: AccessPolicyTargetType[] = ['agent', 'skill', 'agent_type'];
const actionOptions = ['read', 'write', 'execute', 'admin'];

const providerBadge: Record<CredentialProviderType, string> = {
  hashicorp_vault: 'bg-purple-100 text-purple-800',
  aws_sts: 'bg-orange-100 text-orange-800',
  azure_managed_identity: 'bg-blue-100 text-blue-800',
  azure_key_vault: 'bg-sky-100 text-sky-800',
  gcp_workload_identity: 'bg-red-100 text-red-800',
  static: 'bg-gray-100 text-gray-800',
  custom: 'bg-cyan-100 text-cyan-800',
};

const leaseStatusColor: Record<CredentialLeaseStatus, string> = {
  active: 'bg-green-100 text-green-800',
  expired: 'bg-gray-100 text-gray-600',
  revoked: 'bg-red-100 text-red-800',
};

type Tab = 'providers' | 'policies' | 'leases';

function formatTTL(seconds: number): string {
  if (seconds >= 3600) return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`;
  if (seconds >= 60) return `${Math.floor(seconds / 60)}m`;
  return `${seconds}s`;
}

function TimeRemaining({ expiresAt }: { expiresAt: string }) {
  const [remaining, setRemaining] = useState('');

  useEffect(() => {
    const update = () => {
      const diff = new Date(expiresAt).getTime() - Date.now();
      if (diff <= 0) { setRemaining('Expired'); return; }
      const h = Math.floor(diff / 3600000);
      const m = Math.floor((diff % 3600000) / 60000);
      const s = Math.floor((diff % 60000) / 1000);
      setRemaining(`${h}h ${m}m ${s}s`);
    };
    update();
    const interval = setInterval(update, 1000);
    return () => clearInterval(interval);
  }, [expiresAt]);

  return <span className="font-mono text-xs">{remaining}</span>;
}

// -- Create Provider Modal --
function CreateProviderModal({ onSubmit, onClose, isPending }: {
  onSubmit: (data: CreateCredentialProviderRequest) => void; onClose: () => void; isPending: boolean;
}) {
  const [form, setForm] = useState<CreateCredentialProviderRequest>({ name: '', provider_type: 'hashicorp_vault', config: {} });
  const [configJson, setConfigJson] = useState('{}');

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-lg rounded-lg bg-white shadow-xl">
        <div className="flex items-center justify-between border-b border-gray-200 px-6 py-4">
          <h2 className="text-lg font-semibold text-gray-900">Add Credential Provider</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
        </div>
        <form onSubmit={(e) => {
          e.preventDefault();
          try { onSubmit({ ...form, config: JSON.parse(configJson) }); } catch { /* ignore bad json */ }
        }} className="p-6 space-y-4">
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">Name *</label>
            <input required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })}
              className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">Provider Type *</label>
            <select value={form.provider_type} onChange={(e) => setForm({ ...form, provider_type: e.target.value as CredentialProviderType })}
              className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none">
              {providerTypes.map((t) => <option key={t} value={t}>{t.replace(/_/g, ' ')}</option>)}
            </select>
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">Configuration (JSON)</label>
            <textarea rows={6} value={configJson} onChange={(e) => setConfigJson(e.target.value)}
              className="w-full font-mono rounded-md border border-gray-300 px-3 py-2 text-xs focus:border-blue-500 focus:outline-none" />
            {form.provider_type === 'azure_key_vault' && (
              <p className="mt-1 text-xs text-gray-500">
                {'{"vault_url": "https://<vault-name>.vault.azure.net/", "tenant_id": "...", "client_id": "...", "client_secret": "..."}'}
                {' '}-- service-principal (client credentials) auth. Credential path when requesting a lease is the secret name, optionally {'"name/version"'} to pin a version.
              </p>
            )}
          </div>
          <div className="flex justify-end gap-3 border-t border-gray-200 pt-4">
            <button type="button" onClick={onClose} className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
            <button type="submit" disabled={isPending}
              className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">
              {isPending ? 'Creating...' : 'Create Provider'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// -- Create Policy Modal --
function CreatePolicyModal({ onSubmit, onClose, isPending }: {
  onSubmit: (data: CreateAccessPolicyRequest) => void; onClose: () => void; isPending: boolean;
}) {
  const [form, setForm] = useState<CreateAccessPolicyRequest>({
    name: '', target_type: 'agent', target_id: '', resource_type: '', allowed_actions: [], max_ttl_seconds: 3600, require_approval: false,
  });

  const { data: agentsData } = useQuery({ queryKey: ['a2a-agents'], queryFn: () => listA2AAgents() });
  const { data: skillsData } = useQuery({ queryKey: ['skills'], queryFn: () => listSkills() });

  const targets = form.target_type === 'skill'
    ? (skillsData?.data ?? []).map((s) => ({ id: s.id, name: s.name }))
    : form.target_type === 'agent'
      ? (agentsData?.data ?? []).map((a) => ({ id: a.id, name: a.name }))
      : [];

  const toggleAction = (action: string) => {
    setForm({
      ...form,
      allowed_actions: form.allowed_actions.includes(action)
        ? form.allowed_actions.filter((a) => a !== action)
        : [...form.allowed_actions, action],
    });
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-lg rounded-lg bg-white shadow-xl">
        <div className="flex items-center justify-between border-b border-gray-200 px-6 py-4">
          <h2 className="text-lg font-semibold text-gray-900">Create Access Policy</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
        </div>
        <form onSubmit={(e) => { e.preventDefault(); onSubmit(form); }} className="max-h-[70vh] overflow-auto p-6 space-y-4">
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">Name *</label>
            <input required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })}
              className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="mb-1 block text-sm font-medium text-gray-700">Target Type *</label>
              <select value={form.target_type} onChange={(e) => setForm({ ...form, target_type: e.target.value as AccessPolicyTargetType, target_id: '' })}
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none">
                {targetTypes.map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
            </div>
            <div>
              <label className="mb-1 block text-sm font-medium text-gray-700">Target *</label>
              {form.target_type === 'agent_type' ? (
                <input required value={form.target_id} onChange={(e) => setForm({ ...form, target_id: e.target.value })}
                  placeholder="e.g. triage"
                  className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
              ) : (
                <select required value={form.target_id} onChange={(e) => setForm({ ...form, target_id: e.target.value })}
                  className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none">
                  <option value="">Select...</option>
                  {targets.map((t) => <option key={t.id} value={t.id}>{t.name}</option>)}
                </select>
              )}
            </div>
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">Resource Type *</label>
            <input required value={form.resource_type} onChange={(e) => setForm({ ...form, resource_type: e.target.value })}
              placeholder="e.g. database, kubernetes"
              className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
          </div>
          <div>
            <label className="mb-2 block text-sm font-medium text-gray-700">Allowed Actions</label>
            <div className="flex flex-wrap gap-2">
              {actionOptions.map((action) => (
                <label key={action} className={`inline-flex cursor-pointer items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium transition ${
                  form.allowed_actions.includes(action)
                    ? 'border-blue-500 bg-blue-50 text-blue-700'
                    : 'border-gray-300 bg-white text-gray-600 hover:bg-gray-50'
                }`}>
                  <input type="checkbox" className="sr-only" checked={form.allowed_actions.includes(action)} onChange={() => toggleAction(action)} />
                  {action}
                </label>
              ))}
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="mb-1 block text-sm font-medium text-gray-700">Max TTL (seconds)</label>
              <input type="number" value={form.max_ttl_seconds} onChange={(e) => setForm({ ...form, max_ttl_seconds: parseInt(e.target.value) || 0 })}
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
            </div>
            <div className="flex items-end pb-2">
              <label className="inline-flex cursor-pointer items-center gap-2">
                <input type="checkbox" checked={form.require_approval} onChange={(e) => setForm({ ...form, require_approval: e.target.checked })}
                  className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
                <span className="text-sm text-gray-700">Require Approval</span>
              </label>
            </div>
          </div>
          <div className="flex justify-end gap-3 border-t border-gray-200 pt-4">
            <button type="button" onClick={onClose} className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
            <button type="submit" disabled={isPending}
              className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">
              {isPending ? 'Creating...' : 'Create Policy'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// -- Providers Tab --
function ProvidersTab() {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [showModal, setShowModal] = useState(false);

  const { data, isLoading } = useQuery({ queryKey: ['credential-providers'], queryFn: listCredentialProviders });

  const createMut = useMutation({
    mutationFn: createCredentialProvider,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['credential-providers'] }); setShowModal(false); addToast({ type: 'success', title: 'Credential provider created successfully' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to create credential provider', message: err?.response?.data?.error || err.message }); },
  });

  const deleteMut = useMutation({
    mutationFn: deleteCredentialProvider,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['credential-providers'] }); addToast({ type: 'success', title: 'Credential provider deleted' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to delete credential provider', message: err?.response?.data?.error || err.message }); },
  });

  const providers = data?.data ?? [];

  return (
    <div>
      <div className="mb-4 flex justify-end">
        <button onClick={() => setShowModal(true)}
          className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700">
          <Plus className="h-4 w-4" /> Add Provider
        </button>
      </div>

      {isLoading ? <p className="text-sm text-gray-500">Loading...</p> : providers.length === 0 ? (
        <div className="rounded-lg border border-dashed border-gray-300 p-12 text-center">
          <KeyRound className="mx-auto h-12 w-12 text-gray-300" />
          <p className="mt-3 text-sm text-gray-500">No credential providers configured</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {providers.map((provider) => (
            <div key={provider.id} className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-2">
                  <KeyRound className="h-4 w-4 text-gray-400" />
                  <h3 className="font-semibold text-gray-900">{provider.name}</h3>
                </div>
                <button onClick={() => deleteMut.mutate(provider.id)} className="text-red-400 hover:text-red-600">
                  <Trash2 className="h-4 w-4" />
                </button>
              </div>
              <div className="mt-2 flex items-center gap-2">
                <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${providerBadge[provider.provider_type]}`}>
                  {provider.provider_type.replace(/_/g, ' ')}
                </span>
                <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${provider.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'}`}>
                  {provider.enabled ? 'Enabled' : 'Disabled'}
                </span>
              </div>
              <p className="mt-2 text-xs text-gray-400">Created {new Date(provider.created_at).toLocaleDateString()}</p>
            </div>
          ))}
        </div>
      )}

      {showModal && <CreateProviderModal onSubmit={(data) => createMut.mutate(data)} onClose={() => setShowModal(false)} isPending={createMut.isPending} />}
    </div>
  );
}

// -- Policies Tab --
function PoliciesTab() {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [showModal, setShowModal] = useState(false);

  const { data, isLoading } = useQuery({ queryKey: ['access-policies'], queryFn: listAccessPolicies });

  const createMut = useMutation({
    mutationFn: createAccessPolicy,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['access-policies'] }); setShowModal(false); addToast({ type: 'success', title: 'Access policy created successfully' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to create access policy', message: err?.response?.data?.error || err.message }); },
  });

  const deleteMut = useMutation({
    mutationFn: deleteAccessPolicy,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['access-policies'] }); addToast({ type: 'success', title: 'Access policy deleted' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to delete access policy', message: err?.response?.data?.error || err.message }); },
  });

  const policies = data?.data ?? [];

  return (
    <div>
      <div className="mb-4 flex justify-end">
        <button onClick={() => setShowModal(true)}
          className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700">
          <Plus className="h-4 w-4" /> Create Policy
        </button>
      </div>

      {isLoading ? <p className="text-sm text-gray-500">Loading...</p> : policies.length === 0 ? (
        <div className="rounded-lg border border-dashed border-gray-300 p-12 text-center">
          <Shield className="mx-auto h-12 w-12 text-gray-300" />
          <p className="mt-3 text-sm text-gray-500">No access policies defined</p>
        </div>
      ) : (
        <div className="overflow-auto rounded-lg border border-gray-200 bg-white">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Name</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Target</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Resource Type</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Actions</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Max TTL</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Approval</th>
                <th className="px-4 py-3"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {policies.map((policy) => (
                <tr key={policy.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3 text-sm font-medium text-gray-900">{policy.name}</td>
                  <td className="px-4 py-3 text-sm text-gray-600">
                    <span className="rounded bg-gray-100 px-1.5 py-0.5 text-xs">{policy.target_type}</span>{' '}
                    <span className="text-xs text-gray-400">{policy.target_id.substring(0, 8)}...</span>
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-600">{policy.resource_type}</td>
                  <td className="px-4 py-3">
                    <div className="flex flex-wrap gap-1">
                      {policy.allowed_actions.map((a) => (
                        <span key={a} className="rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">{a}</span>
                      ))}
                    </div>
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-600">{formatTTL(policy.max_ttl_seconds)}</td>
                  <td className="px-4 py-3">
                    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${policy.require_approval ? 'bg-amber-100 text-amber-800' : 'bg-gray-100 text-gray-600'}`}>
                      {policy.require_approval ? 'Required' : 'No'}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <button onClick={() => deleteMut.mutate(policy.id)} className="text-red-400 hover:text-red-600">
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showModal && <CreatePolicyModal onSubmit={(data) => createMut.mutate(data)} onClose={() => setShowModal(false)} isPending={createMut.isPending} />}
    </div>
  );
}

// -- Leases Tab --
function LeasesTab() {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [revoking, setRevoking] = useState<string | null>(null);

  const { data, isLoading } = useQuery({ queryKey: ['credential-leases'], queryFn: () => listCredentialLeases() });

  const revokeMut = useMutation({
    mutationFn: revokeCredentialLease,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['credential-leases'] }); setRevoking(null); addToast({ type: 'success', title: 'Credential lease revoked' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to revoke credential lease', message: err?.response?.data?.error || err.message }); },
  });

  const leases = data?.data ?? [];

  return (
    <div>
      {isLoading ? <p className="text-sm text-gray-500">Loading...</p> : leases.length === 0 ? (
        <div className="rounded-lg border border-dashed border-gray-300 p-12 text-center">
          <Clock className="mx-auto h-12 w-12 text-gray-300" />
          <p className="mt-3 text-sm text-gray-500">No credential leases</p>
        </div>
      ) : (
        <div className="overflow-auto rounded-lg border border-gray-200 bg-white">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Incident</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Agent</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Skill</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Reason</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Issued</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Expires</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Remaining</th>
                <th className="px-4 py-3"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {leases.map((lease) => (
                <tr key={lease.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3">
                    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${leaseStatusColor[lease.status]}`}>
                      {lease.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-xs text-gray-600 font-mono">{lease.incident_id ? lease.incident_id.substring(0, 8) + '...' : '-'}</td>
                  <td className="px-4 py-3 text-xs text-gray-600 font-mono">{lease.agent_id ? lease.agent_id.substring(0, 8) + '...' : '-'}</td>
                  <td className="px-4 py-3 text-xs text-gray-600 font-mono">{lease.skill_id ? lease.skill_id.substring(0, 8) + '...' : '-'}</td>
                  <td className="px-4 py-3 text-sm text-gray-600 max-w-48 truncate">{lease.request_reason}</td>
                  <td className="px-4 py-3 text-xs text-gray-500">{new Date(lease.issued_at).toLocaleString()}</td>
                  <td className="px-4 py-3 text-xs text-gray-500">{new Date(lease.expires_at).toLocaleString()}</td>
                  <td className="px-4 py-3">
                    {lease.status === 'active' ? <TimeRemaining expiresAt={lease.expires_at} /> : <span className="text-xs text-gray-400">-</span>}
                  </td>
                  <td className="px-4 py-3">
                    {lease.status === 'active' && (
                      revoking === lease.id ? (
                        <div className="flex items-center gap-1">
                          <button onClick={() => revokeMut.mutate(lease.id)}
                            className="rounded bg-red-600 px-2 py-1 text-xs text-white hover:bg-red-700">Confirm</button>
                          <button onClick={() => setRevoking(null)}
                            className="rounded bg-gray-100 px-2 py-1 text-xs text-gray-700 hover:bg-gray-200">Cancel</button>
                        </div>
                      ) : (
                        <button onClick={() => setRevoking(lease.id)}
                          className="rounded bg-red-50 px-2 py-1 text-xs font-medium text-red-700 hover:bg-red-100">Revoke</button>
                      )
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

// -- Main Page --
export function CredentialsPage() {
  const [tab, setTab] = useState<Tab>('providers');

  const tabs: { key: Tab; label: string; icon: typeof KeyRound }[] = [
    { key: 'providers', label: 'Providers', icon: KeyRound },
    { key: 'policies', label: 'Access Policies', icon: Shield },
    { key: 'leases', label: 'Active Leases', icon: Clock },
  ];

  return (
    <div className="p-8">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Credential Management</h1>
        <p className="mt-1 text-sm text-gray-500">Manage credential providers, access policies, and active leases</p>
      </div>

      <div className="mt-6 flex gap-0 border-b border-gray-200">
        {tabs.map(({ key, label, icon: Icon }) => (
          <button key={key} onClick={() => setTab(key)}
            className={`inline-flex items-center gap-1.5 border-b-2 px-4 py-2.5 text-sm font-medium transition-colors ${
              tab === key ? 'border-blue-600 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-700'
            }`}>
            <Icon className="h-4 w-4" />{label}
          </button>
        ))}
      </div>

      <div className="mt-6">
        {tab === 'providers' && <ProvidersTab />}
        {tab === 'policies' && <PoliciesTab />}
        {tab === 'leases' && <LeasesTab />}
      </div>
    </div>
  );
}
