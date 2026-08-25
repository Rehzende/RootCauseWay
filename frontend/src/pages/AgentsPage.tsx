import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  Plus, Trash2, X, Activity, Globe, Shield, Clock, ArrowLeft, Bot, Puzzle, Heart, RefreshCw,
} from 'lucide-react';
import {
  listA2AAgents, createA2AAgent, updateA2AAgent, deleteA2AAgent,
  listSoftware, listSkills, listAgentSkills, linkSkillToAgent, unlinkSkillFromAgent,
  healthCheckA2AAgent, healthCheckAllA2AAgents,
} from '@/services/api';
import { useToast } from '@/components/Toast';
import { PermissionGate } from '@/components/PermissionGate';
import { PermissionButton } from '@/components/PermissionButton';
import type {
  A2AAgent, CreateA2AAgentRequest, A2AAgentType, A2AAuthType, AgentHostingType, LLMProvider,
  AgentSkill, A2AHealthStatus, Skill, AgentSkillLink,
} from '@/types/api';

const agentTypes: A2AAgentType[] = ['triage', 'evidence_analysis', 'rca', 'postmortem', 'custom'];
const authTypes: A2AAuthType[] = ['none', 'bearer', 'api_key'];

const typeBorderColor: Record<A2AAgentType, string> = {
  triage: 'border-l-amber-500',
  evidence_analysis: 'border-l-blue-500',
  rca: 'border-l-purple-500',
  postmortem: 'border-l-green-500',
  custom: 'border-l-gray-500',
};

const typeBadgeColor: Record<A2AAgentType, string> = {
  triage: 'bg-amber-100 text-amber-800',
  evidence_analysis: 'bg-blue-100 text-blue-800',
  rca: 'bg-purple-100 text-purple-800',
  postmortem: 'bg-green-100 text-green-800',
  custom: 'bg-gray-100 text-gray-800',
};

function HealthDot({ status }: { status: A2AHealthStatus }) {
  const color = status === 'healthy' ? 'bg-green-500' : status === 'unhealthy' ? 'bg-red-500' : 'bg-gray-400';
  return <span className={`inline-block h-2.5 w-2.5 rounded-full ${color}`} title={status} />;
}

// -- Agent Card --
function AgentCard({
  agent, onToggle, onDelete, onClick, onHealthCheck,
}: {
  agent: A2AAgent;
  onToggle: () => void;
  onDelete: () => void;
  onClick: () => void;
  onHealthCheck: () => void;
}) {
  return (
    <div
      onClick={onClick}
      className={`cursor-pointer rounded-lg border border-l-4 bg-white p-5 shadow-sm transition hover:shadow-md ${typeBorderColor[agent.agent_type] ?? 'border-l-gray-400'}`}
    >
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-2">
          <HealthDot status={agent.health_status} />
          <h3 className="font-semibold text-gray-900">{agent.name}</h3>
          <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${typeBadgeColor[agent.agent_type] ?? 'bg-gray-100 text-gray-800'}`}>
            {agent.agent_type.replace(/_/g, ' ')}
          </span>
          {agent.is_system && <span className="rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700">system</span>}
          <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${agent.hosting_type === 'byoa' ? 'bg-purple-100 text-purple-800' : 'bg-blue-100 text-blue-800'}`}>
            {agent.hosting_type === 'byoa' ? 'BYOA' : 'Managed'}
          </span>
        </div>
        <div className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
          {!agent.is_system && (
            <label className="relative inline-flex cursor-pointer items-center">
              <input type="checkbox" checked={agent.enabled} onChange={onToggle} className="peer sr-only" />
              <div className="peer h-5 w-9 rounded-full bg-gray-300 after:absolute after:left-[2px] after:top-[2px] after:h-4 after:w-4 after:rounded-full after:bg-white after:transition-all peer-checked:bg-blue-600 peer-checked:after:translate-x-full" />
            </label>
          )}
          {!agent.is_system && (
            <PermissionGate resource="agents" action="write">
              <button onClick={onDelete} className="text-red-400 hover:text-red-600"><Trash2 className="h-4 w-4" /></button>
            </PermissionGate>
          )}
        </div>
      </div>

      {agent.description && <p className="mt-2 text-sm text-gray-600 line-clamp-2">{agent.description}</p>}

      <div className="mt-3 flex items-center gap-1 text-xs text-gray-500">
        <Globe className="h-3 w-3" />
        <span className="truncate">{agent.endpoint_url || (agent.hosting_type === 'managed' ? 'Managed by RootCauseway' : 'No endpoint')}</span>
      </div>
      <div className="mt-1 flex items-center gap-2 text-xs text-gray-400">
        <span>LLM: {agent.llm_provider === 'custom' ? 'Custom' : 'Platform'}</span>
        {agent.hosting_type === 'managed' && agent.auto_scale && (
          <span>Auto-scale: {agent.min_replicas}-{agent.max_replicas}</span>
        )}
      </div>

      {agent.skills.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-1">
          {agent.skills.map((s) => (
            <span key={s.id} className="rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">
              {s.name}
            </span>
          ))}
        </div>
      )}

      <div className="mt-3 flex items-center justify-between">
        {agent.last_health_check ? (
          <div className="flex items-center gap-1 text-xs text-gray-400">
            <Clock className="h-3 w-3" />
            Last check: {new Date(agent.last_health_check).toLocaleString()}
          </div>
        ) : (
          <span className="text-xs text-gray-400">No health check yet</span>
        )}
        <button
          onClick={(e) => { e.stopPropagation(); onHealthCheck(); }}
          className="inline-flex items-center gap-1 rounded border border-gray-200 px-2 py-1 text-xs text-gray-500 hover:bg-gray-50 hover:text-gray-700"
        >
          <Heart className="h-3 w-3" /> Check
        </button>
      </div>
    </div>
  );
}

// -- Create Modal --
function emptyForm(): CreateA2AAgentRequest {
  return { name: '', agent_type: 'triage', endpoint_url: '', auth_type: 'none', skills: [], allowed_software_ids: [], hosting_type: 'managed', llm_provider: 'platform' };
}

function CreateAgentModal({
  onSubmit, onClose, isPending,
}: {
  onSubmit: (data: CreateA2AAgentRequest) => void;
  onClose: () => void;
  isPending: boolean;
}) {
  const [form, setForm] = useState<CreateA2AAgentRequest & { auth_credentials?: string }>(emptyForm());

  const { data: softwareData } = useQuery({
    queryKey: ['software', 1],
    queryFn: () => listSoftware(1, 100),
  });

  const addSkill = () => {
    setForm({ ...form, skills: [...(form.skills ?? []), { id: crypto.randomUUID(), name: '' }] });
  };
  const removeSkill = (i: number) => {
    setForm({ ...form, skills: (form.skills ?? []).filter((_, idx) => idx !== i) });
  };
  const updateSkill = (i: number, field: keyof AgentSkill, v: string) => {
    setForm({ ...form, skills: (form.skills ?? []).map((s, idx) => idx === i ? { ...s, [field]: v } : s) });
  };

  const toggleSoftware = (id: string) => {
    const ids = form.allowed_software_ids ?? [];
    setForm({
      ...form,
      allowed_software_ids: ids.includes(id) ? ids.filter((x) => x !== id) : [...ids, id],
    });
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-2xl rounded-lg bg-white shadow-xl">
        <div className="flex items-center justify-between border-b border-gray-200 px-6 py-4">
          <h2 className="text-lg font-semibold text-gray-900">Add A2A Agent</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
        </div>

        <form onSubmit={(e) => { e.preventDefault(); onSubmit(form); }} className="max-h-[70vh] overflow-auto p-6 space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="mb-1 block text-sm font-medium text-gray-700">Name *</label>
              <input required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })}
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
            </div>
            <div>
              <label className="mb-1 block text-sm font-medium text-gray-700">Agent Type *</label>
              <select value={form.agent_type} onChange={(e) => setForm({ ...form, agent_type: e.target.value as A2AAgentType })}
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none">
                {agentTypes.map((t) => <option key={t} value={t}>{t.replace(/_/g, ' ')}</option>)}
              </select>
            </div>
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">Description</label>
            <textarea rows={2} value={form.description ?? ''} onChange={(e) => setForm({ ...form, description: e.target.value })}
              className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
          </div>

          {/* Hosting Type */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="mb-1 block text-sm font-medium text-gray-700">Hosting Type</label>
              <select value={form.hosting_type ?? 'managed'} onChange={(e) => {
                const ht = e.target.value as AgentHostingType;
                setForm({ ...form, hosting_type: ht, llm_provider: ht === 'byoa' ? 'custom' : 'platform' });
              }}
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none">
                <option value="managed">Managed (RootCauseway hosts)</option>
                <option value="byoa">BYOA (Bring Your Own Agent)</option>
              </select>
            </div>
            <div>
              <label className="mb-1 block text-sm font-medium text-gray-700">LLM Provider</label>
              <select value={form.llm_provider ?? 'platform'} onChange={(e) => setForm({ ...form, llm_provider: e.target.value as LLMProvider })}
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none">
                <option value="platform">Platform (RootCauseway key)</option>
                <option value="custom">Custom (Bring your own)</option>
              </select>
            </div>
          </div>

          {form.hosting_type === 'byoa' && (
            <div className="rounded-md border border-purple-200 bg-purple-50 p-3 text-sm text-purple-800">
              Deploy this agent in your infrastructure and provide the endpoint URL below.
            </div>
          )}

          {form.llm_provider === 'custom' && (
            <div>
              <label className="mb-1 block text-sm font-medium text-gray-700">LLM API Key Reference</label>
              <input value={(form as any).llm_api_key_ref ?? ''} onChange={(e) => setForm({ ...form, llm_api_key_ref: e.target.value })}
                placeholder="credential-store/path/to/key"
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
            </div>
          )}

          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">
              Endpoint URL {form.hosting_type === 'byoa' ? '*' : '(optional for managed)'}
            </label>
            <input required={form.hosting_type === 'byoa'} value={form.endpoint_url} onChange={(e) => setForm({ ...form, endpoint_url: e.target.value })}
              placeholder="https://agent.example.com/.well-known/agent"
              className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="mb-1 block text-sm font-medium text-gray-700">Authentication</label>
              <select value={form.auth_type ?? 'none'} onChange={(e) => setForm({ ...form, auth_type: e.target.value as A2AAuthType })}
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none">
                {authTypes.map((t) => <option key={t} value={t}>{t.replace('_', ' ')}</option>)}
              </select>
            </div>
            {form.auth_type !== 'none' && (
              <div>
                <label className="mb-1 block text-sm font-medium text-gray-700">Credentials</label>
                <input type="password" value={form.auth_credentials ?? ''} onChange={(e) => setForm({ ...form, auth_credentials: e.target.value })}
                  className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
              </div>
            )}
          </div>

          {/* Skills */}
          <div>
            <div className="mb-2 flex items-center justify-between">
              <label className="text-sm font-medium text-gray-700">Skills</label>
              <button type="button" onClick={addSkill} className="text-xs text-blue-600 hover:underline">+ Add Skill</button>
            </div>
            {(form.skills ?? []).map((s, i) => (
              <div key={i} className="mb-2 flex gap-2">
                <input placeholder="Name" value={s.name} onChange={(e) => updateSkill(i, 'name', e.target.value)}
                  className="flex-1 rounded-md border border-gray-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none" />
                <input placeholder="Description" value={s.description ?? ''} onChange={(e) => updateSkill(i, 'description', e.target.value)}
                  className="flex-1 rounded-md border border-gray-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none" />
                <button type="button" onClick={() => removeSkill(i)} className="text-red-400 hover:text-red-600">
                  <X className="h-4 w-4" />
                </button>
              </div>
            ))}
          </div>

          {/* Allowed Software */}
          {(softwareData?.data ?? []).length > 0 && (
            <div>
              <label className="mb-2 block text-sm font-medium text-gray-700">Allowed Software</label>
              <div className="flex flex-wrap gap-2">
                {softwareData!.data.map((sw) => (
                  <label key={sw.id} className={`inline-flex cursor-pointer items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium transition ${
                    (form.allowed_software_ids ?? []).includes(sw.id)
                      ? 'border-blue-500 bg-blue-50 text-blue-700'
                      : 'border-gray-300 bg-white text-gray-600 hover:bg-gray-50'
                  }`}>
                    <input type="checkbox" className="sr-only" checked={(form.allowed_software_ids ?? []).includes(sw.id)}
                      onChange={() => toggleSoftware(sw.id)} />
                    {sw.name}
                  </label>
                ))}
              </div>
            </div>
          )}

          <div className="flex justify-end gap-3 border-t border-gray-200 pt-4">
            <button type="button" onClick={onClose} className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
            <button type="submit" disabled={isPending}
              className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">
              {isPending ? 'Creating...' : 'Create Agent'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// -- Agent Detail --
function AgentDetail({ agent, onBack }: { agent: A2AAgent; onBack: () => void }) {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const existingConfig = (agent.managed_config ?? {}) as { model?: string; temperature?: number };
  const [model, setModel] = useState(existingConfig.model ?? '');
  const [temperature, setTemperature] = useState(
    existingConfig.temperature != null ? String(existingConfig.temperature) : '',
  );

  const overrideMut = useMutation({
    mutationFn: () => {
      const managed_config: Record<string, unknown> = {};
      if (model.trim()) managed_config.model = model.trim();
      if (temperature.trim()) managed_config.temperature = Number(temperature);
      // Update() layers every one of these fields onto the existing row
      // unconditionally (see backend a2a_service.go) -- omitting name/
      // agent_type/endpoint_url here would blank them out, not just leave
      // them untouched, so the full identity must be re-sent alongside the
      // one field this form actually changes.
      return updateA2AAgent(agent.id, {
        name: agent.name, description: agent.description, agent_type: agent.agent_type,
        endpoint_url: agent.endpoint_url, managed_config,
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['a2a-agents'] });
      addToast({ type: 'success', title: 'LLM override saved' });
    },
    onError: (err: any) => {
      addToast({ type: 'error', title: 'Failed to save LLM override', message: err?.response?.data?.error || err.message });
    },
  });

  return (
    <div className="p-8">
      <button onClick={onBack} className="mb-4 inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-700">
        <ArrowLeft className="h-4 w-4" /> Back to Agents
      </button>
      <div className="flex items-center gap-3">
        <HealthDot status={agent.health_status} />
        <h1 className="text-2xl font-bold text-gray-900">{agent.name}</h1>
        <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${typeBadgeColor[agent.agent_type]}`}>
          {agent.agent_type.replace(/_/g, ' ')}
        </span>
        <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${agent.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'}`}>
          {agent.enabled ? 'Enabled' : 'Disabled'}
        </span>
        <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${agent.hosting_type === 'byoa' ? 'bg-purple-100 text-purple-800' : 'bg-blue-100 text-blue-800'}`}>
          {agent.hosting_type === 'byoa' ? 'BYOA' : 'Managed'}
        </span>
      </div>
      {agent.description && <p className="mt-2 text-sm text-gray-600">{agent.description}</p>}

      <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-2">
        <div className="rounded-lg border border-gray-200 bg-white p-5">
          <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-900"><Globe className="h-4 w-4 text-gray-400" /> Endpoint & Hosting</h3>
          <p className="text-sm text-gray-700 break-all">{agent.endpoint_url || (agent.hosting_type === 'managed' ? 'Managed by RootCauseway' : 'No endpoint configured')}</p>
          <p className="mt-2 text-xs text-gray-500">Auth: <span className="font-medium">{agent.auth_type}</span></p>
          <p className="mt-1 text-xs text-gray-500">LLM Provider: <span className="font-medium">{agent.llm_provider === 'custom' ? 'Custom (BYOA key)' : 'Platform (RootCauseway)'}</span></p>
          {agent.hosting_type === 'managed' && (
            <div className="mt-2 text-xs text-gray-500">
              <p>Auto-scale: <span className="font-medium">{agent.auto_scale ? `${agent.min_replicas}-${agent.max_replicas} replicas` : 'Off'}</span></p>
            </div>
          )}
          {agent.last_health_check && (
            <p className="mt-2 flex items-center gap-1 text-xs text-gray-400">
              <Clock className="h-3 w-3" /> Last check: {new Date(agent.last_health_check).toLocaleString()}
            </p>
          )}
        </div>

        <div className="rounded-lg border border-gray-200 bg-white p-5">
          <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-900"><Activity className="h-4 w-4 text-gray-400" /> Skills</h3>
          {agent.skills.length > 0 ? (
            <div className="space-y-2">
              {agent.skills.map((s) => (
                <div key={s.id} className="rounded-md bg-blue-50 px-3 py-2">
                  <p className="text-sm font-medium text-blue-800">{s.name}</p>
                  {s.description && <p className="text-xs text-blue-600">{s.description}</p>}
                </div>
              ))}
            </div>
          ) : <p className="text-sm text-gray-400">No skills defined</p>}
        </div>

        {agent.hosting_type === 'managed' && agent.llm_provider === 'platform' && (
          <div className="rounded-lg border border-gray-200 bg-white p-5">
            <h3 className="mb-1 text-sm font-semibold text-gray-900">LLM Override</h3>
            <p className="mb-3 text-xs text-gray-500">
              Overrides the org's default model/temperature (set on the{' '}
              <a href="/settings" className="text-blue-600 hover:underline">Settings → LLM &amp; Tokens</a> tab)
              for this agent only. Leave blank to inherit the org default.
            </p>
            <form onSubmit={(e) => { e.preventDefault(); overrideMut.mutate(); }} className="space-y-3">
              <div>
                <label className="block text-xs font-medium text-gray-500">Model</label>
                <input
                  value={model}
                  onChange={(e) => setModel(e.target.value)}
                  placeholder="Inherit org default"
                  className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-500">Temperature</label>
                <input
                  type="number" step="0.1" min="0" max="2"
                  value={temperature}
                  onChange={(e) => setTemperature(e.target.value)}
                  placeholder="Inherit agent default"
                  className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm"
                />
              </div>
              <PermissionGate resource="agents" action="write">
                <button
                  type="submit"
                  disabled={overrideMut.isPending}
                  className="rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-700 disabled:opacity-50"
                >
                  {overrideMut.isPending ? 'Saving...' : 'Save override'}
                </button>
              </PermissionGate>
            </form>
          </div>
        )}

        <div className="lg:col-span-2 rounded-lg border border-gray-200 bg-white p-5">
          <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-900"><Shield className="h-4 w-4 text-gray-400" /> Agent Card</h3>
          <pre className="overflow-auto rounded-md bg-gray-50 p-4 text-xs text-gray-700">
            {JSON.stringify(agent.agent_card, null, 2)}
          </pre>
        </div>
      </div>
    </div>
  );
}

// -- Manage Registry Skills Modal --
function ManageSkillsModal({ agent, onClose }: { agent: A2AAgent; onClose: () => void }) {
  const queryClient = useQueryClient();
  const { data: skillsData } = useQuery({ queryKey: ['skills'], queryFn: () => listSkills() });
  const { data: agentLinks, isLoading: linksLoading } = useQuery({
    queryKey: ['agent-skills', agent.id],
    queryFn: () => listAgentSkills(agent.id),
  });

  const linkMut = useMutation({
    mutationFn: (skillId: string) => linkSkillToAgent(agent.id, skillId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['agent-skills', agent.id] }),
  });
  const unlinkMut = useMutation({
    mutationFn: (skillId: string) => unlinkSkillFromAgent(agent.id, skillId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['agent-skills', agent.id] }),
  });

  const linkedIds = new Set((agentLinks ?? []).map((l: AgentSkillLink) => l.skill_id));
  const allSkills = skillsData?.data ?? [];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-lg rounded-lg bg-white shadow-xl">
        <div className="flex items-center justify-between border-b border-gray-200 px-6 py-4">
          <h2 className="text-lg font-semibold text-gray-900">Manage Skills for "{agent.name}"</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
        </div>
        <div className="max-h-[60vh] overflow-auto p-6 space-y-2">
          {linksLoading ? <p className="text-sm text-gray-500">Loading...</p> : allSkills.length === 0 ? (
            <p className="text-sm text-gray-500">No skills in registry. Create skills first.</p>
          ) : allSkills.map((skill: Skill) => (
            <div key={skill.id} className="flex items-center justify-between rounded-md border border-gray-200 px-4 py-3">
              <div className="flex items-center gap-2">
                <Puzzle className="h-4 w-4 text-gray-400" />
                <div>
                  <p className="text-sm font-medium text-gray-900">{skill.name}</p>
                  <p className="text-xs text-gray-500">{skill.category}</p>
                </div>
              </div>
              <PermissionButton
                resource="skills" action="write"
                onClick={() => linkedIds.has(skill.id) ? unlinkMut.mutate(skill.id) : linkMut.mutate(skill.id)}
                className={`rounded-md px-3 py-1.5 text-xs font-medium ${
                  linkedIds.has(skill.id) ? 'bg-red-50 text-red-700 hover:bg-red-100' : 'bg-blue-50 text-blue-700 hover:bg-blue-100'
                }`}
              >
                {linkedIds.has(skill.id) ? 'Unlink' : 'Link'}
              </PermissionButton>
            </div>
          ))}
        </div>
        <div className="border-t border-gray-200 px-6 py-4">
          <button onClick={onClose} className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Close</button>
        </div>
      </div>
    </div>
  );
}

// -- Main Page --
export function AgentsPage() {
  const queryClient = useQueryClient();
  const [showModal, setShowModal] = useState(false);
  const [selectedAgent, setSelectedAgent] = useState<A2AAgent | null>(null);
  const [skillsAgent, setSkillsAgent] = useState<A2AAgent | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ['a2a-agents'],
    queryFn: () => listA2AAgents(),
    refetchInterval: 60000,
  });

  const { addToast } = useToast();

  const createMut = useMutation({
    mutationFn: createA2AAgent,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['a2a-agents'] }); setShowModal(false); addToast({ type: 'success', title: 'Agent created successfully' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to create agent', message: err?.response?.data?.error || err.message }); },
  });

  const toggleMut = useMutation({
    mutationFn: ({ id, enabled, agent }: { id: string; enabled: boolean; agent: A2AAgent }) =>
      updateA2AAgent(id, { name: agent.name, description: agent.description, agent_type: agent.agent_type, endpoint_url: agent.endpoint_url, enabled }),
    onSuccess: (_, vars) => { queryClient.invalidateQueries({ queryKey: ['a2a-agents'] }); addToast({ type: 'success', title: `Agent ${vars.enabled ? 'enabled' : 'disabled'}` }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to update agent', message: err?.response?.data?.error || err.message }); },
  });

  const deleteMut = useMutation({
    mutationFn: deleteA2AAgent,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['a2a-agents'] }); addToast({ type: 'success', title: 'Agent deleted' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to delete agent', message: err?.response?.data?.error || err.message }); },
  });

  const healthCheckMut = useMutation({
    mutationFn: healthCheckA2AAgent,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['a2a-agents'] }); addToast({ type: 'success', title: 'Health check complete' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Health check failed', message: err?.response?.data?.error || err.message }); },
  });

  const healthCheckAllMut = useMutation({
    mutationFn: healthCheckAllA2AAgents,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['a2a-agents'] }); addToast({ type: 'success', title: 'All agents health checked' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Health check failed', message: err?.response?.data?.error || err.message }); },
  });

  if (selectedAgent) {
    return <AgentDetail agent={selectedAgent} onBack={() => setSelectedAgent(null)} />;
  }

  const agents = data?.data ?? [];

  return (
    <div className="p-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">A2A Agents</h1>
          <p className="mt-1 text-sm text-gray-500">Manage Agent-to-Agent protocol agents for incident analysis</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => healthCheckAllMut.mutate()}
            disabled={healthCheckAllMut.isPending}
            className="inline-flex items-center gap-2 rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
          >
            <RefreshCw className={`h-4 w-4 ${healthCheckAllMut.isPending ? 'animate-spin' : ''}`} /> Check All
          </button>
          <PermissionGate resource="agents" action="write">
            <button
              onClick={() => setShowModal(true)}
              className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
            >
              <Plus className="h-4 w-4" /> Add Agent
            </button>
          </PermissionGate>
        </div>
      </div>

      <div className="mt-6">
        {isLoading ? (
          <p className="text-sm text-gray-500">Loading...</p>
        ) : agents.length === 0 ? (
          <div className="rounded-lg border border-dashed border-gray-300 p-12 text-center">
            <Bot className="mx-auto h-12 w-12 text-gray-300" />
            <p className="mt-3 text-sm text-gray-500">No A2A agents configured yet</p>
            <button onClick={() => setShowModal(true)}
              className="mt-3 inline-flex items-center gap-1 text-sm font-medium text-blue-600 hover:underline">
              <Plus className="h-4 w-4" /> Add your first agent
            </button>
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
            {agents.map((agent) => (
              <div key={agent.id}>
                <AgentCard
                  agent={agent}
                  onToggle={() => toggleMut.mutate({ id: agent.id, enabled: !agent.enabled, agent })}
                  onDelete={() => { if (window.confirm(`Delete agent "${agent.name}"? This cannot be undone.`)) deleteMut.mutate(agent.id); }}
                  onClick={() => setSelectedAgent(agent)}
                  onHealthCheck={() => healthCheckMut.mutate(agent.id)}
                />
                <div className="mt-1 px-1">
                  <button
                    onClick={() => setSkillsAgent(agent)}
                    className="inline-flex items-center gap-1 text-xs text-purple-600 hover:underline"
                  >
                    <Puzzle className="h-3 w-3" /> Manage Skills
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {showModal && (
        <CreateAgentModal
          onSubmit={(data) => createMut.mutate(data)}
          onClose={() => setShowModal(false)}
          isPending={createMut.isPending}
        />
      )}

      {skillsAgent && (
        <ManageSkillsModal agent={skillsAgent} onClose={() => setSkillsAgent(null)} />
      )}
    </div>
  );
}
