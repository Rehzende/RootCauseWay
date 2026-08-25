import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  Plus, Trash2, X, Cloud, Database,
  Users, BarChart3, Link2, Tag, Server, ArrowLeft, KeyRound, ShieldCheck, Activity, Pencil,
} from 'lucide-react';
import { useToast } from '@/components/Toast';
import {
  listSoftware, createSoftware, updateSoftware, deleteSoftware, getSoftwareSummary,
  listResourceCredentials, createResourceCredential, deleteResourceCredential,
  listCredentialProviders,
} from '@/services/api';
import { DataTable, type Column } from '@/components/DataTable';
import { criticalityBadge, softwareTypeLabel, dependencyRelationLabel } from '@/lib/software';
import type {
  SoftwareEntry, CreateSoftwareRequest, CloudProvider, Person,
  CloudResource, DatabaseInfo, ResourceCredential, CreateResourceCredentialRequest,
  SoftwareCriticality, SoftwareType, SoftwareDependency, DependencyRelation,
} from '@/types/api';

const cloudProviders: CloudProvider[] = ['aws', 'azure', 'gcp', 'on_prem', 'hybrid'];
const criticalities: SoftwareCriticality[] = ['critical', 'high', 'medium', 'low'];
const softwareTypes: SoftwareType[] = ['service', 'library', 'database', 'job', 'website', 'other'];
const dependencyRelations: DependencyRelation[] = ['depends_on', 'uses_api_of', 'shares_database_with'];

const cloudBadge: Record<string, string> = {
  aws: 'bg-orange-100 text-orange-800',
  azure: 'bg-blue-100 text-blue-800',
  gcp: 'bg-red-100 text-red-800',
  on_prem: 'bg-gray-100 text-gray-800',
  hybrid: 'bg-purple-100 text-purple-800',
};

// -- Reusable sub-components --

function PersonListEditor({
  label, value, onChange,
}: {
  label: string;
  value: Person[];
  onChange: (v: Person[]) => void;
}) {
  const add = () => onChange([...value, { name: '', email: '' }]);
  const remove = (i: number) => onChange(value.filter((_, idx) => idx !== i));
  const update = (i: number, field: keyof Person, v: string) =>
    onChange(value.map((p, idx) => (idx === i ? { ...p, [field]: v } : p)));

  return (
    <div>
      <div className="mb-2 flex items-center justify-between">
        <label className="text-sm font-medium text-gray-700">{label}</label>
        <button type="button" onClick={add} className="text-xs text-blue-600 hover:underline">+ Add</button>
      </div>
      {value.map((p, i) => (
        <div key={i} className="mb-2 flex gap-2">
          <input placeholder="Name" value={p.name} onChange={(e) => update(i, 'name', e.target.value)}
            className="flex-1 rounded-md border border-gray-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none" />
          <input placeholder="Email" value={p.email} onChange={(e) => update(i, 'email', e.target.value)}
            className="flex-1 rounded-md border border-gray-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none" />
          <input placeholder="Role" value={p.role ?? ''} onChange={(e) => update(i, 'role', e.target.value)}
            className="w-28 rounded-md border border-gray-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none" />
          <button type="button" onClick={() => remove(i)} className="text-red-400 hover:text-red-600">
            <X className="h-4 w-4" />
          </button>
        </div>
      ))}
    </div>
  );
}

function StringListEditor({
  label, value, onChange, placeholder,
}: {
  label: string;
  value: string[];
  onChange: (v: string[]) => void;
  placeholder?: string;
}) {
  const [input, setInput] = useState('');
  const add = () => {
    if (input.trim() && !value.includes(input.trim())) {
      onChange([...value, input.trim()]);
      setInput('');
    }
  };
  return (
    <div>
      <label className="mb-1 block text-sm font-medium text-gray-700">{label}</label>
      <div className="mb-2 flex flex-wrap gap-1">
        {value.map((v) => (
          <span key={v} className="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-700">
            {v}
            <button type="button" onClick={() => onChange(value.filter((x) => x !== v))} className="text-gray-400 hover:text-red-500">
              <X className="h-3 w-3" />
            </button>
          </span>
        ))}
      </div>
      <div className="flex gap-2">
        <input value={input} onChange={(e) => setInput(e.target.value)} placeholder={placeholder ?? 'Add item'}
          onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); add(); } }}
          className="flex-1 rounded-md border border-gray-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none" />
        <button type="button" onClick={add} className="rounded-md bg-gray-100 px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-200">Add</button>
      </div>
    </div>
  );
}

// -- Form defaults --
function emptyForm(): CreateSoftwareRequest {
  return {
    name: '', slug: '', description: '', repository_url: '', pipeline_url: '',
    cloud_provider: undefined, cloud_resources: [], database_info: [], infra_details: {},
    stakeholders: [], sre_team: [], architects: [],
    runbook_url: '', dashboard_url: '', dependencies: [], tags: [],
    criticality: 'medium', type: 'service',
  };
}

function formFromEntry(s: SoftwareEntry): CreateSoftwareRequest {
  return {
    name: s.name, slug: s.slug, description: s.description,
    repository_url: s.repository_url, pipeline_url: s.pipeline_url,
    cloud_provider: s.cloud_provider, cloud_resources: s.cloud_resources ?? [],
    database_info: s.database_info ?? [], infra_details: s.infra_details ?? {},
    stakeholders: s.stakeholders ?? [],
    sre_team: s.sre_team ?? [], architects: s.architects ?? [],
    runbook_url: s.runbook_url, dashboard_url: s.dashboard_url,
    dependencies: s.dependencies ?? [], tags: s.tags ?? [],
    criticality: s.criticality ?? 'medium', type: s.type ?? 'service',
  };
}

// -- Dependency editor: pick an existing catalog entry + a relation type,
// rather than free-typing a slug (typos silently produced a dependency that
// never resolved to anything -- see SoftwareService.GetDependencyGraph's
// "skips unresolved" behavior, which made a typo indistinguishable from an
// intentional external/unregistered dependency).
function DependencyListEditor({
  value, onChange, options, selfSlug,
}: {
  value: SoftwareDependency[];
  onChange: (v: SoftwareDependency[]) => void;
  options: SoftwareEntry[];
  selfSlug?: string;
}) {
  const [slug, setSlug] = useState('');
  const [relation, setRelation] = useState<DependencyRelation>('depends_on');
  const available = options.filter((o) => o.slug !== selfSlug && !value.some((d) => d.slug === o.slug));

  const add = () => {
    if (!slug) return;
    onChange([...value, { slug, relation }]);
    setSlug('');
    setRelation('depends_on');
  };
  const remove = (s: string) => onChange(value.filter((d) => d.slug !== s));

  return (
    <div>
      <label className="mb-1 block text-sm font-medium text-gray-700">Dependencies</label>
      <div className="mb-2 flex flex-wrap gap-1">
        {value.map((d) => (
          <span key={d.slug} className="inline-flex items-center gap-1 rounded-full bg-blue-50 px-2 py-0.5 text-xs text-blue-700">
            {d.slug} <span className="text-blue-400">({dependencyRelationLabel[d.relation] ?? d.relation})</span>
            <button type="button" onClick={() => remove(d.slug)} className="text-blue-400 hover:text-red-500">
              <X className="h-3 w-3" />
            </button>
          </span>
        ))}
        {value.length === 0 && <span className="text-sm text-gray-400">No dependencies mapped</span>}
      </div>
      <div className="flex gap-2">
        <select value={slug} onChange={(e) => setSlug(e.target.value)}
          className="flex-1 rounded-md border border-gray-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none">
          <option value="">Select software...</option>
          {available.map((o) => <option key={o.id} value={o.slug}>{o.name} ({o.slug})</option>)}
        </select>
        <select value={relation} onChange={(e) => setRelation(e.target.value as DependencyRelation)}
          className="w-44 rounded-md border border-gray-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none">
          {dependencyRelations.map((r) => <option key={r} value={r}>{dependencyRelationLabel[r]}</option>)}
        </select>
        <button type="button" onClick={add} className="rounded-md bg-gray-100 px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-200">Add</button>
      </div>
      {available.length === 0 && (
        <p className="mt-1 text-xs text-gray-400">
          No other registered software available -- an external/unregistered dependency can't be mapped here yet.
        </p>
      )}
    </div>
  );
}

// -- Modal --
type ModalMode = 'create' | 'edit';

function SoftwareModal({
  mode, form, setForm, onSubmit, onClose, isPending, allSoftware,
}: {
  mode: ModalMode;
  form: CreateSoftwareRequest;
  setForm: (f: CreateSoftwareRequest) => void;
  onSubmit: () => void;
  onClose: () => void;
  isPending: boolean;
  allSoftware: SoftwareEntry[];
}) {
  const [section, setSection] = useState<'basic' | 'infra' | 'team' | 'ops' | 'tags'>('basic');
  const sections = [
    { key: 'basic' as const, label: 'Basic Info', icon: Server },
    { key: 'infra' as const, label: 'Infrastructure', icon: Cloud },
    { key: 'team' as const, label: 'Team', icon: Users },
    { key: 'ops' as const, label: 'Operations', icon: BarChart3 },
    { key: 'tags' as const, label: 'Tags', icon: Tag },
  ];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-3xl rounded-lg bg-white shadow-xl">
        <div className="flex items-center justify-between border-b border-gray-200 px-6 py-4">
          <h2 className="text-lg font-semibold text-gray-900">
            {mode === 'create' ? 'Add Software' : 'Edit Software'}
          </h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
        </div>

        {/* Section tabs */}
        <div className="flex gap-0 border-b border-gray-200 px-6">
          {sections.map(({ key, label, icon: Icon }) => (
            <button key={key} onClick={() => setSection(key)}
              className={`inline-flex items-center gap-1.5 border-b-2 px-3 py-2.5 text-xs font-medium transition-colors ${
                section === key ? 'border-blue-600 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-700'
              }`}>
              <Icon className="h-3.5 w-3.5" />{label}
            </button>
          ))}
        </div>

        <form onSubmit={(e) => { e.preventDefault(); onSubmit(); }} className="max-h-[60vh] overflow-auto p-6">
          {section === 'basic' && (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="mb-1 block text-sm font-medium text-gray-700">Name *</label>
                  <input required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })}
                    className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
                </div>
                <div>
                  <label className="mb-1 block text-sm font-medium text-gray-700">Slug *</label>
                  <input required pattern="^[a-z0-9-]+$" value={form.slug} onChange={(e) => setForm({ ...form, slug: e.target.value })}
                    className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
                </div>
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-gray-700">Description</label>
                <textarea rows={3} value={form.description ?? ''} onChange={(e) => setForm({ ...form, description: e.target.value })}
                  className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="mb-1 block text-sm font-medium text-gray-700">Repository URL</label>
                  <input value={form.repository_url ?? ''} onChange={(e) => setForm({ ...form, repository_url: e.target.value })}
                    className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
                </div>
                <div>
                  <label className="mb-1 block text-sm font-medium text-gray-700">Pipeline URL</label>
                  <input value={form.pipeline_url ?? ''} onChange={(e) => setForm({ ...form, pipeline_url: e.target.value })}
                    className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
                </div>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="mb-1 block text-sm font-medium text-gray-700">Criticality</label>
                  <p className="mb-1 text-xs text-gray-400">Business-impact tier -- drives default incident severity/escalation priority.</p>
                  <select value={form.criticality ?? 'medium'} onChange={(e) => setForm({ ...form, criticality: e.target.value as SoftwareCriticality })}
                    className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none">
                    {criticalities.map((c) => <option key={c} value={c}>{c.charAt(0).toUpperCase() + c.slice(1)}</option>)}
                  </select>
                </div>
                <div>
                  <label className="mb-1 block text-sm font-medium text-gray-700">Type</label>
                  <select value={form.type ?? 'service'} onChange={(e) => setForm({ ...form, type: e.target.value as SoftwareType })}
                    className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none">
                    {softwareTypes.map((t) => <option key={t} value={t}>{softwareTypeLabel[t]}</option>)}
                  </select>
                </div>
              </div>
            </div>
          )}

          {section === 'infra' && (
            <div className="space-y-4">
              <div>
                <label className="mb-1 block text-sm font-medium text-gray-700">Cloud Provider</label>
                <select value={form.cloud_provider ?? ''} onChange={(e) => setForm({ ...form, cloud_provider: (e.target.value || undefined) as CloudProvider | undefined })}
                  className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none">
                  <option value="">Select...</option>
                  {cloudProviders.map((cp) => <option key={cp} value={cp}>{cp.replace('_', ' ').toUpperCase()}</option>)}
                </select>
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-gray-700">Cloud Resources (JSON)</label>
                <textarea rows={4} value={JSON.stringify(form.cloud_resources ?? [], null, 2)}
                  onChange={(e) => { try { setForm({ ...form, cloud_resources: JSON.parse(e.target.value) as CloudResource[] }); } catch { /* ignore parse errors while typing */ } }}
                  className="w-full font-mono rounded-md border border-gray-300 px-3 py-2 text-xs focus:border-blue-500 focus:outline-none" />
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-gray-700">Database Info (JSON)</label>
                <textarea rows={4} value={JSON.stringify(form.database_info ?? [], null, 2)}
                  onChange={(e) => { try { setForm({ ...form, database_info: JSON.parse(e.target.value) as DatabaseInfo[] }); } catch { /* ignore */ } }}
                  className="w-full font-mono rounded-md border border-gray-300 px-3 py-2 text-xs focus:border-blue-500 focus:outline-none" />
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-gray-700">Infra Details (JSON)</label>
                <p className="mb-1 text-xs text-gray-400">
                  Free-form: cluster name, namespace, instance sizing, region — anything an RCA needs to know
                  about where this actually runs.
                </p>
                <textarea rows={4} value={JSON.stringify(form.infra_details ?? {}, null, 2)}
                  onChange={(e) => { try { setForm({ ...form, infra_details: JSON.parse(e.target.value) as Record<string, unknown> }); } catch { /* ignore */ } }}
                  className="w-full font-mono rounded-md border border-gray-300 px-3 py-2 text-xs focus:border-blue-500 focus:outline-none" />
              </div>
            </div>
          )}

          {section === 'team' && (
            <div className="space-y-6">
              <PersonListEditor label="Stakeholders" value={form.stakeholders ?? []}
                onChange={(v) => setForm({ ...form, stakeholders: v })} />
              <PersonListEditor label="SRE Team" value={form.sre_team ?? []}
                onChange={(v) => setForm({ ...form, sre_team: v })} />
              <PersonListEditor label="Architects" value={form.architects ?? []}
                onChange={(v) => setForm({ ...form, architects: v })} />
            </div>
          )}

          {section === 'ops' && (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="mb-1 block text-sm font-medium text-gray-700">Runbook URL</label>
                  <input value={form.runbook_url ?? ''} onChange={(e) => setForm({ ...form, runbook_url: e.target.value })}
                    className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
                </div>
                <div>
                  <label className="mb-1 block text-sm font-medium text-gray-700">Dashboard URL</label>
                  <input value={form.dashboard_url ?? ''} onChange={(e) => setForm({ ...form, dashboard_url: e.target.value })}
                    className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
                </div>
              </div>
              <DependencyListEditor value={form.dependencies ?? []} selfSlug={form.slug}
                onChange={(v) => setForm({ ...form, dependencies: v })} options={allSoftware} />
            </div>
          )}

          {section === 'tags' && (
            <StringListEditor label="Tags" value={form.tags ?? []}
              onChange={(v) => setForm({ ...form, tags: v })} placeholder="e.g. production, critical" />
          )}

          <div className="mt-6 flex justify-end gap-3 border-t border-gray-200 pt-4">
            <button type="button" onClick={onClose} className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
            <button type="submit" disabled={isPending}
              className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">
              {isPending ? 'Saving...' : mode === 'create' ? 'Create' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// -- Add Resource Credential Modal --
function AddCredentialModal({ softwareId, onClose }: { softwareId: string; onClose: () => void }) {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [form, setForm] = useState<Omit<CreateResourceCredentialRequest, 'software_id'>>({
    resource_name: '', resource_type: '', provider_id: '', credential_path: '', default_ttl_seconds: 3600, max_ttl_seconds: 86400,
  });

  const { data: providersData } = useQuery({ queryKey: ['credential-providers'], queryFn: listCredentialProviders });

  const createMut = useMutation({
    mutationFn: (data: CreateResourceCredentialRequest) => createResourceCredential(data),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['resource-credentials', softwareId] }); onClose(); addToast({ type: 'success', title: 'Resource credential added successfully' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to add resource credential', message: err?.response?.data?.error || err.message }); },
  });

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-lg rounded-lg bg-white shadow-xl">
        <div className="flex items-center justify-between border-b border-gray-200 px-6 py-4">
          <h2 className="text-lg font-semibold text-gray-900">Add Resource Credential</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
        </div>
        <form onSubmit={(e) => { e.preventDefault(); createMut.mutate({ ...form, software_id: softwareId }); }} className="p-6 space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="mb-1 block text-sm font-medium text-gray-700">Resource Name *</label>
              <input required value={form.resource_name} onChange={(e) => setForm({ ...form, resource_name: e.target.value })}
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
            </div>
            <div>
              <label className="mb-1 block text-sm font-medium text-gray-700">Resource Type *</label>
              <select required value={form.resource_type} onChange={(e) => setForm({ ...form, resource_type: e.target.value })}
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none">
                <option value="">Select...</option>
                {['database', 'kubernetes', 'cloud_api', 'ssh', 'service_account', 'custom'].map((t) => (
                  <option key={t} value={t}>{t.replace(/_/g, ' ')}</option>
                ))}
              </select>
            </div>
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">Provider *</label>
            <select required value={form.provider_id} onChange={(e) => setForm({ ...form, provider_id: e.target.value })}
              className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none">
              <option value="">Select provider...</option>
              {(providersData?.data ?? []).map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
            </select>
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">Credential Path *</label>
            <input required value={form.credential_path} onChange={(e) => setForm({ ...form, credential_path: e.target.value })}
              placeholder="e.g. secret/data/myapp/db"
              className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="mb-1 block text-sm font-medium text-gray-700">Default TTL (seconds)</label>
              <input type="number" value={form.default_ttl_seconds} onChange={(e) => setForm({ ...form, default_ttl_seconds: parseInt(e.target.value) || 0 })}
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
            </div>
            <div>
              <label className="mb-1 block text-sm font-medium text-gray-700">Max TTL (seconds)</label>
              <input type="number" value={form.max_ttl_seconds} onChange={(e) => setForm({ ...form, max_ttl_seconds: parseInt(e.target.value) || 0 })}
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
            </div>
          </div>
          <div className="flex justify-end gap-3 border-t border-gray-200 pt-4">
            <button type="button" onClick={onClose} className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
            <button type="submit" disabled={createMut.isPending}
              className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">
              {createMut.isPending ? 'Adding...' : 'Add Credential'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// -- Resource Credentials Section --
function ResourceCredentialsSection({ softwareId }: { softwareId: string }) {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [showModal, setShowModal] = useState(false);

  const { data: credentials } = useQuery({
    queryKey: ['resource-credentials', softwareId],
    queryFn: () => listResourceCredentials(softwareId),
  });

  const { data: providersData } = useQuery({ queryKey: ['credential-providers'], queryFn: listCredentialProviders });
  const providers = providersData?.data ?? [];
  const providerName = (id: string) => providers.find((p) => p.id === id)?.name ?? id.substring(0, 8) + '...';

  const deleteMut = useMutation({
    mutationFn: deleteResourceCredential,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['resource-credentials', softwareId] }); addToast({ type: 'success', title: 'Resource credential deleted' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to delete resource credential', message: err?.response?.data?.error || err.message }); },
  });

  const resourceTypeBadge: Record<string, string> = {
    database: 'bg-purple-100 text-purple-800',
    kubernetes: 'bg-blue-100 text-blue-800',
    cloud_api: 'bg-orange-100 text-orange-800',
    ssh: 'bg-green-100 text-green-800',
    service_account: 'bg-cyan-100 text-cyan-800',
    custom: 'bg-gray-100 text-gray-800',
  };

  const formatTTL = (seconds: number) => {
    if (seconds >= 3600) return `${Math.floor(seconds / 3600)}h`;
    if (seconds >= 60) return `${Math.floor(seconds / 60)}m`;
    return `${seconds}s`;
  };

  return (
    <div className="lg:col-span-2 rounded-lg border border-gray-200 bg-white p-5">
      <div className="flex items-center justify-between mb-3">
        <h3 className="flex items-center gap-2 text-sm font-semibold text-gray-900">
          <KeyRound className="h-4 w-4 text-gray-400" /> Resource Credentials
        </h3>
        <button onClick={() => setShowModal(true)}
          className="inline-flex items-center gap-1 text-xs font-medium text-blue-600 hover:underline">
          <Plus className="h-3 w-3" /> Add Credential
        </button>
      </div>
      {(credentials ?? []).length > 0 ? (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          {(credentials ?? []).map((cred: ResourceCredential) => (
            <div key={cred.id} className="rounded-md border border-gray-200 p-3">
              <div className="flex items-start justify-between">
                <div>
                  <p className="text-sm font-medium text-gray-900">{cred.resource_name}</p>
                  <span className={`mt-1 inline-block rounded-full px-2 py-0.5 text-xs font-medium ${resourceTypeBadge[cred.resource_type] ?? 'bg-gray-100 text-gray-800'}`}>
                    {cred.resource_type.replace(/_/g, ' ')}
                  </span>
                </div>
                <button onClick={() => deleteMut.mutate(cred.id)} className="text-red-400 hover:text-red-600">
                  <Trash2 className="h-3.5 w-3.5" />
                </button>
              </div>
              <div className="mt-2 space-y-1 text-xs text-gray-500">
                <p>Provider: <span className="font-medium text-gray-700">{providerName(cred.provider_id)}</span></p>
                <p>TTL: {formatTTL(cred.default_ttl_seconds)} (max {formatTTL(cred.max_ttl_seconds)})</p>
              </div>
            </div>
          ))}
        </div>
      ) : <p className="text-sm text-gray-400">No resource credentials configured</p>}

      {showModal && <AddCredentialModal softwareId={softwareId} onClose={() => setShowModal(false)} />}
    </div>
  );
}

// -- Completeness + reliability rollup card --
function CompletenessCard({ softwareId }: { softwareId: string }) {
  const { data: summary, isLoading } = useQuery({
    queryKey: ['software-summary', softwareId],
    queryFn: () => getSoftwareSummary(softwareId),
  });

  if (isLoading || !summary) {
    return (
      <div className="rounded-lg border border-gray-200 bg-white p-5">
        <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-900"><ShieldCheck className="h-4 w-4 text-gray-400" /> Completeness & Reliability</h3>
        <p className="text-sm text-gray-400">Loading...</p>
      </div>
    );
  }

  const pct = summary.completeness_total > 0 ? Math.round((summary.completeness_score / summary.completeness_total) * 100) : 0;
  const barColor = pct >= 80 ? 'bg-green-500' : pct >= 50 ? 'bg-yellow-500' : 'bg-red-500';

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-5">
      <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-900"><ShieldCheck className="h-4 w-4 text-gray-400" /> Completeness & Reliability</h3>

      <div className="mb-4">
        <div className="mb-1 flex items-center justify-between text-xs text-gray-500">
          <span>Catalog completeness</span>
          <span className="font-medium text-gray-700">{summary.completeness_score}/{summary.completeness_total}</span>
        </div>
        <div className="h-2 w-full rounded-full bg-gray-100">
          <div className={`h-2 rounded-full ${barColor}`} style={{ width: `${pct}%` }} />
        </div>
        {summary.missing_checks.length > 0 && (
          <div className="mt-2 flex flex-wrap gap-1">
            {summary.missing_checks.map((m) => (
              <span key={m} className="rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-500">missing: {m.replace(/_/g, ' ')}</span>
            ))}
          </div>
        )}
      </div>

      <div className="grid grid-cols-2 gap-3 text-sm md:grid-cols-4">
        <div>
          <p className="text-xs uppercase text-gray-400">SLOs</p>
          <p className="font-semibold text-gray-900">{summary.slo_count}</p>
        </div>
        <div>
          <p className="text-xs uppercase text-gray-400">Escalation policies</p>
          <p className="font-semibold text-gray-900">{summary.escalation_policy_count}</p>
        </div>
        <div>
          <p className="text-xs uppercase text-gray-400">Open incidents</p>
          <p className={`font-semibold ${summary.open_incidents > 0 ? 'text-red-600' : 'text-gray-900'}`}>{summary.open_incidents}</p>
        </div>
        <div>
          <p className="text-xs uppercase text-gray-400">Total incidents</p>
          <p className="font-semibold text-gray-900">{summary.total_incidents}</p>
        </div>
      </div>
      {summary.last_incident_at && (
        <p className="mt-3 flex items-center gap-1 text-xs text-gray-400">
          <Activity className="h-3 w-3" /> Last incident {new Date(summary.last_incident_at).toLocaleDateString()}
        </p>
      )}
    </div>
  );
}

// -- Detail view --
function SoftwareDetail({ software, onBack, onEdit }: { software: SoftwareEntry; onBack: () => void; onEdit: (s: SoftwareEntry) => void }) {
  return (
    <div className="p-8">
      <button onClick={onBack} className="mb-4 inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-700">
        <ArrowLeft className="h-4 w-4" /> Back to Catalog
      </button>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="text-2xl font-bold text-gray-900">{software.name}</h1>
          <code className="rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-600">{software.slug}</code>
          <span className="rounded-full bg-indigo-50 px-2 py-0.5 text-xs font-medium text-indigo-700">
            {softwareTypeLabel[software.type] ?? software.type}
          </span>
          <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${criticalityBadge[software.criticality] ?? 'bg-gray-100 text-gray-700'}`}>
            {software.criticality} criticality
          </span>
          <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${
            software.status === 'active' ? 'bg-green-100 text-green-800' :
            software.status === 'deprecated' ? 'bg-yellow-100 text-yellow-800' : 'bg-gray-100 text-gray-800'
          }`}>{software.status}</span>
          {software.cloud_provider && (
            <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${cloudBadge[software.cloud_provider] ?? 'bg-gray-100 text-gray-800'}`}>
              {software.cloud_provider.replace('_', ' ').toUpperCase()}
            </span>
          )}
        </div>
        <button onClick={() => onEdit(software)}
          className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700">
          <Pencil className="h-4 w-4" /> Edit
        </button>
      </div>
      {software.description && <p className="mt-2 text-sm text-gray-600">{software.description}</p>}

      <div className="mt-6">
        <CompletenessCard softwareId={software.id} />
      </div>

      <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* Links */}
        <div className="rounded-lg border border-gray-200 bg-white p-5">
          <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-900"><Link2 className="h-4 w-4 text-gray-400" /> Links</h3>
          <div className="space-y-2 text-sm">
            {software.repository_url && <div><span className="text-gray-500">Repository:</span> <a href={software.repository_url} target="_blank" rel="noreferrer" className="text-blue-600 hover:underline">{software.repository_url}</a></div>}
            {software.pipeline_url && <div><span className="text-gray-500">Pipeline:</span> <a href={software.pipeline_url} target="_blank" rel="noreferrer" className="text-blue-600 hover:underline">{software.pipeline_url}</a></div>}
            {software.runbook_url && <div><span className="text-gray-500">Runbook:</span> <a href={software.runbook_url} target="_blank" rel="noreferrer" className="text-blue-600 hover:underline">{software.runbook_url}</a></div>}
            {software.dashboard_url && <div><span className="text-gray-500">Dashboard:</span> <a href={software.dashboard_url} target="_blank" rel="noreferrer" className="text-blue-600 hover:underline">{software.dashboard_url}</a></div>}
          </div>
        </div>

        {/* Infrastructure */}
        <div className="rounded-lg border border-gray-200 bg-white p-5">
          <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-900"><Cloud className="h-4 w-4 text-gray-400" /> Infrastructure</h3>
          {(software.cloud_resources ?? []).length > 0 ? (
            <div className="space-y-1">
              {software.cloud_resources!.map((r, i) => (
                <div key={i} className="flex items-center gap-2 text-sm">
                  <span className="rounded bg-gray-100 px-1.5 py-0.5 text-xs font-mono text-gray-600">{r.type}</span>
                  <span className="text-gray-800">{r.name}</span>
                  {r.region && <span className="text-xs text-gray-400">{r.region}</span>}
                </div>
              ))}
            </div>
          ) : <p className="text-sm text-gray-400">No resources listed</p>}
          {(software.database_info ?? []).length > 0 && (
            <div className="mt-3">
              <p className="mb-1 text-xs font-medium text-gray-500">Databases</p>
              {software.database_info!.map((db, i) => (
                <div key={i} className="flex items-center gap-2 text-sm">
                  <Database className="h-3.5 w-3.5 text-gray-400" />
                  <span className="font-medium text-gray-800">{db.name}</span>
                  <span className="text-xs text-gray-500">{db.engine ?? db.type}</span>
                </div>
              ))}
            </div>
          )}
          {software.infra_details && Object.keys(software.infra_details).length > 0 && (
            <div className="mt-3">
              <p className="mb-1 text-xs font-medium text-gray-500">Infra Details</p>
              <div className="space-y-0.5">
                {Object.entries(software.infra_details).map(([k, v]) => (
                  <div key={k} className="flex gap-2 text-sm">
                    <span className="text-gray-500">{k}:</span>
                    <span className="text-gray-800">{String(v)}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        {/* Team */}
        <div className="rounded-lg border border-gray-200 bg-white p-5">
          <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-900"><Users className="h-4 w-4 text-gray-400" /> Team</h3>
          {[
            { label: 'SRE Team', list: software.sre_team },
            { label: 'Stakeholders', list: software.stakeholders },
            { label: 'Architects', list: software.architects },
          ].map(({ label, list }) => (list ?? []).length > 0 && (
            <div key={label} className="mb-3">
              <p className="mb-1 text-xs font-medium text-gray-500">{label}</p>
              {list!.map((p, i) => (
                <div key={i} className="text-sm text-gray-700">
                  {p.name} <span className="text-gray-400">({p.email})</span>
                  {p.role && <span className="ml-1 rounded bg-gray-100 px-1 text-xs text-gray-500">{p.role}</span>}
                </div>
              ))}
            </div>
          ))}
        </div>

        {/* Dependencies & Tags */}
        <div className="rounded-lg border border-gray-200 bg-white p-5">
          <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-900"><Tag className="h-4 w-4 text-gray-400" /> Dependencies & Tags</h3>
          {(software.dependencies ?? []).length > 0 && (
            <div className="mb-3">
              <p className="mb-1 text-xs font-medium text-gray-500">Dependencies</p>
              <div className="flex flex-wrap gap-1">
                {software.dependencies!.map((d) => (
                  <span key={d.slug} className="rounded bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">
                    {d.slug} <span className="font-normal text-blue-500">({dependencyRelationLabel[d.relation] ?? d.relation})</span>
                  </span>
                ))}
              </div>
            </div>
          )}
          <div>
            <p className="mb-1 text-xs font-medium text-gray-500">Tags</p>
            <div className="flex flex-wrap gap-1">
              {(software.tags ?? []).map((t) => (
                <span key={t} className="rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-600">{t}</span>
              ))}
              {!(software.tags ?? []).length && <span className="text-sm text-gray-400">No tags</span>}
            </div>
          </div>
        </div>

        <ResourceCredentialsSection softwareId={software.id} />
      </div>
    </div>
  );
}

// -- Main Page --
export function SoftwarePage() {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [page, setPage] = useState(1);
  const [modalMode, setModalMode] = useState<ModalMode | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState<CreateSoftwareRequest>(emptyForm());
  const [selectedSoftware, setSelectedSoftware] = useState<SoftwareEntry | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ['software', page],
    queryFn: () => listSoftware(page),
  });

  // Full-ish list (not just the current page) for the dependency picker --
  // a dependency should be pickable regardless of which page it happens to
  // sort into in the main table.
  const { data: allSoftwareData } = useQuery({
    queryKey: ['software', 'all-for-dependency-picker'],
    queryFn: () => listSoftware(1, 200),
  });

  const createMut = useMutation({
    mutationFn: createSoftware,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['software'] });
      setModalMode(null);
      setForm(emptyForm());
      addToast({ type: 'success', title: 'Software created successfully' });
    },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to create software', message: err?.response?.data?.error || err.message }); },
  });

  const updateMut = useMutation({
    mutationFn: (data: CreateSoftwareRequest) => updateSoftware(editingId!, data),
    onSuccess: (updated) => {
      queryClient.invalidateQueries({ queryKey: ['software'] });
      // Editing from the detail view keeps selectedSoftware pointing at the
      // pre-edit object -- without this it silently kept showing stale data
      // after a successful save until you left and re-opened the detail.
      setSelectedSoftware((prev) => (prev && prev.id === updated.id ? updated : prev));
      setModalMode(null);
      setEditingId(null);
      setForm(emptyForm());
      addToast({ type: 'success', title: 'Software updated successfully' });
    },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to update software', message: err?.response?.data?.error || err.message }); },
  });

  const deleteMut = useMutation({
    mutationFn: deleteSoftware,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['software'] }); addToast({ type: 'success', title: 'Software deleted' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to delete software', message: err?.response?.data?.error || err.message }); },
  });

  const handleEdit = (s: SoftwareEntry) => {
    setEditingId(s.id);
    setForm(formFromEntry(s));
    setModalMode('edit');
  };

  if (selectedSoftware) {
    return <SoftwareDetail software={selectedSoftware} onBack={() => setSelectedSoftware(null)} onEdit={handleEdit} />;
  }

  const columns: Column<SoftwareEntry>[] = [
    { key: 'name', header: 'Name', render: (s) => <span className="font-medium text-gray-900">{s.name}</span> },
    { key: 'slug', header: 'Slug', render: (s) => <code className="text-xs text-gray-600">{s.slug}</code> },
    { key: 'type', header: 'Type', render: (s) => (
      <span className="rounded-full bg-indigo-50 px-2 py-0.5 text-xs font-medium text-indigo-700">{softwareTypeLabel[s.type] ?? s.type}</span>
    )},
    { key: 'criticality', header: 'Criticality', render: (s) => (
      <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${criticalityBadge[s.criticality] ?? 'bg-gray-100 text-gray-700'}`}>{s.criticality}</span>
    )},
    { key: 'cloud', header: 'Cloud Provider', render: (s) => s.cloud_provider ? (
      <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${cloudBadge[s.cloud_provider] ?? 'bg-gray-100 text-gray-800'}`}>
        {s.cloud_provider.replace('_', ' ').toUpperCase()}
      </span>
    ) : <span className="text-gray-400">-</span> },
    { key: 'status', header: 'Status', render: (s) => (
      <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${
        s.status === 'active' ? 'bg-green-100 text-green-800' :
        s.status === 'deprecated' ? 'bg-yellow-100 text-yellow-800' : 'bg-gray-100 text-gray-800'
      }`}>{s.status}</span>
    )},
    { key: 'sre', header: 'SRE Team', render: (s) => (
      <span className="text-sm text-gray-600">{(s.sre_team ?? []).length || '-'}</span>
    )},
    { key: 'deps', header: 'Dependencies', render: (s) => (
      <span className="text-sm text-gray-600">{(s.dependencies ?? []).length || '-'}</span>
    )},
    { key: 'actions', header: '', render: (s) => (
      <div className="flex items-center gap-2">
        <button onClick={(e) => { e.stopPropagation(); handleEdit(s); }} className="text-gray-400 hover:text-blue-600 text-xs">Edit</button>
        <button onClick={(e) => { e.stopPropagation(); deleteMut.mutate(s.id); }} className="text-red-400 hover:text-red-600">
          <Trash2 className="h-4 w-4" />
        </button>
      </div>
    )},
  ];

  return (
    <div className="p-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Software Catalog</h1>
          <p className="mt-1 text-sm text-gray-500">Manage your registered software entries with infrastructure and team details</p>
        </div>
        <button
          onClick={() => { setForm(emptyForm()); setModalMode('create'); }}
          className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
        >
          <Plus className="h-4 w-4" /> Add Software
        </button>
      </div>

      <div className="mt-6">
        {isLoading ? (
          <p className="text-sm text-gray-500">Loading...</p>
        ) : (
          <DataTable
            columns={columns}
            data={data?.data ?? []}
            total={data?.total ?? 0}
            page={page}
            perPage={data?.per_page ?? 20}
            onPageChange={setPage}
            keyExtractor={(s) => s.id}
            onRowClick={(s) => setSelectedSoftware(s)}
          />
        )}
      </div>

      {modalMode && (
        <SoftwareModal
          mode={modalMode}
          form={form}
          setForm={setForm}
          onSubmit={() => modalMode === 'create' ? createMut.mutate(form) : updateMut.mutate(form)}
          onClose={() => { setModalMode(null); setEditingId(null); setForm(emptyForm()); }}
          isPending={createMut.isPending || updateMut.isPending}
          allSoftware={allSoftwareData?.data ?? []}
        />
      )}
    </div>
  );
}
