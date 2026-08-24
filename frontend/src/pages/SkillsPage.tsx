import { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Plus, Trash2, X, ArrowLeft, Puzzle, Link2, Pencil } from 'lucide-react';
import { useToast } from '@/components/Toast';
import {
  listSkills, createSkill, deleteSkill, updateSkill, getSkill,
  listA2AAgents, linkSkillToAgent, unlinkSkillFromAgent, listAgentSkills,
} from '@/services/api';
import type { Skill, CreateSkillRequest, SkillCategory, A2AAgent } from '@/types/api';

// Everything CreateSkillRequest/updateSkill's payload needs, always
// resent in full on every update -- SkillService.Update (Go) overwrites
// name/slug/description/category/prompt_template unconditionally from
// whatever's in the request, matching AgentsPage.tsx's updateA2AAgent
// pattern. Sending only the one field that changed (e.g. just `enabled`)
// would silently blank out the rest.
function toUpdatePayload(skill: Skill): CreateSkillRequest & { enabled: boolean } {
  return {
    name: skill.name,
    slug: skill.slug,
    description: skill.description,
    category: skill.category,
    prompt_template: skill.prompt_template,
    required_resource_types: skill.required_resource_types ?? [],
    required_permissions: skill.required_permissions ?? [],
    enabled: skill.enabled,
  };
}

const categories: SkillCategory[] = [
  'infrastructure', 'application', 'database', 'network', 'security', 'cloud', 'observability', 'custom',
];

const categoryBorderColor: Record<SkillCategory, string> = {
  infrastructure: 'border-l-blue-500',
  application: 'border-l-green-500',
  database: 'border-l-purple-500',
  network: 'border-l-orange-500',
  security: 'border-l-red-500',
  cloud: 'border-l-cyan-500',
  observability: 'border-l-amber-500',
  custom: 'border-l-gray-500',
};

const categoryBadgeColor: Record<SkillCategory, string> = {
  infrastructure: 'bg-blue-100 text-blue-800',
  application: 'bg-green-100 text-green-800',
  database: 'bg-purple-100 text-purple-800',
  network: 'bg-orange-100 text-orange-800',
  security: 'bg-red-100 text-red-800',
  cloud: 'bg-cyan-100 text-cyan-800',
  observability: 'bg-amber-100 text-amber-800',
  custom: 'bg-gray-100 text-gray-800',
};

function emptyForm(): CreateSkillRequest {
  return { name: '', slug: '', description: '', category: 'custom', prompt_template: '', required_resource_types: [], required_permissions: [] };
}

// -- Skill Card --
function SkillCard({ skill, onDelete, onClick }: { skill: Skill; onDelete: () => void; onClick: () => void }) {
  return (
    <div
      onClick={onClick}
      className={`cursor-pointer rounded-lg border border-l-4 bg-white p-5 shadow-sm transition hover:shadow-md ${categoryBorderColor[skill.category] ?? 'border-l-gray-400'}`}
    >
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-2">
          <Puzzle className="h-4 w-4 text-gray-400" />
          <h3 className="font-semibold text-gray-900">{skill.name}</h3>
          <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${categoryBadgeColor[skill.category] ?? 'bg-gray-100 text-gray-800'}`}>
            {skill.category}
          </span>
        </div>
        <div className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
          <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${skill.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'}`}>
            {skill.enabled ? 'Enabled' : 'Disabled'}
          </span>
          <button onClick={onDelete} className="text-red-400 hover:text-red-600"><Trash2 className="h-4 w-4" /></button>
        </div>
      </div>

      {skill.description && <p className="mt-2 text-sm text-gray-600 line-clamp-2">{skill.description}</p>}

      {(skill.required_resource_types ?? []).length > 0 && (
        <div className="mt-3 flex flex-wrap gap-1">
          {skill.required_resource_types!.map((rt) => (
            <span key={rt} className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">{rt}</span>
          ))}
        </div>
      )}
    </div>
  );
}

// -- String list input --
function StringListInput({ label, value, onChange, placeholder }: {
  label: string; value: string[]; onChange: (v: string[]) => void; placeholder?: string;
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

// -- Create/Edit Skill Modal --
function SkillFormModal({ skill, onSubmit, onClose, isPending }: {
  skill?: Skill; onSubmit: (data: CreateSkillRequest) => void; onClose: () => void; isPending: boolean;
}) {
  const isEdit = !!skill;
  const [form, setForm] = useState<CreateSkillRequest>(() => skill ? {
    name: skill.name,
    slug: skill.slug,
    description: skill.description,
    category: skill.category,
    prompt_template: skill.prompt_template,
    required_resource_types: skill.required_resource_types ?? [],
    required_permissions: skill.required_permissions ?? [],
  } : emptyForm());

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-2xl rounded-lg bg-white shadow-xl">
        <div className="flex items-center justify-between border-b border-gray-200 px-6 py-4">
          <h2 className="text-lg font-semibold text-gray-900">{isEdit ? 'Edit Skill' : 'Create Skill'}</h2>
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
              <label className="mb-1 block text-sm font-medium text-gray-700">Slug *</label>
              <input required pattern="^[a-z0-9-]+$" disabled={isEdit} title={isEdit ? 'Slug cannot be changed after creation' : undefined}
                value={form.slug} onChange={(e) => setForm({ ...form, slug: e.target.value })}
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none disabled:bg-gray-50 disabled:text-gray-500" />
            </div>
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">Description</label>
            <textarea rows={2} value={form.description ?? ''} onChange={(e) => setForm({ ...form, description: e.target.value })}
              className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">Category *</label>
            <select value={form.category} onChange={(e) => setForm({ ...form, category: e.target.value })}
              className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none">
              {categories.map((c) => <option key={c} value={c}>{c}</option>)}
            </select>
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">Prompt Template</label>
            <textarea rows={4} value={form.prompt_template ?? ''} onChange={(e) => setForm({ ...form, prompt_template: e.target.value })}
              className="w-full font-mono rounded-md border border-gray-300 px-3 py-2 text-xs focus:border-blue-500 focus:outline-none" />
          </div>

          <StringListInput label="Required Resource Types" value={form.required_resource_types ?? []}
            onChange={(v) => setForm({ ...form, required_resource_types: v })} placeholder="e.g. database, kubernetes" />

          <StringListInput label="Required Permissions" value={form.required_permissions ?? []}
            onChange={(v) => setForm({ ...form, required_permissions: v })} placeholder="e.g. read, execute" />

          <div className="flex justify-end gap-3 border-t border-gray-200 pt-4">
            <button type="button" onClick={onClose} className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
            <button type="submit" disabled={isPending}
              className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">
              {isPending ? 'Saving...' : isEdit ? 'Save Changes' : 'Create Skill'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// -- Link Agents Modal --
function LinkAgentsModal({ skill, onClose }: { skill: Skill; onClose: () => void }) {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const { data: agentsData } = useQuery({ queryKey: ['a2a-agents'], queryFn: () => listA2AAgents() });
  const { data: linkedSkills } = useQuery({
    queryKey: ['skill-agents', skill.id],
    queryFn: async () => {
      const agents = agentsData?.data ?? [];
      const links: { agent: A2AAgent; linked: boolean }[] = [];
      for (const agent of agents) {
        try {
          const agentSkills = await listAgentSkills(agent.id);
          links.push({ agent, linked: agentSkills.some((l) => l.skill_id === skill.id) });
        } catch {
          links.push({ agent, linked: false });
        }
      }
      return links;
    },
    enabled: !!agentsData,
  });

  const linkMut = useMutation({
    mutationFn: ({ agentId }: { agentId: string }) => linkSkillToAgent(agentId, skill.id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['skill-agents', skill.id] }); addToast({ type: 'success', title: 'Skill linked to agent' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to link skill to agent', message: err?.response?.data?.error || err.message }); },
  });

  const unlinkMut = useMutation({
    mutationFn: ({ agentId }: { agentId: string }) => unlinkSkillFromAgent(agentId, skill.id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['skill-agents', skill.id] }); addToast({ type: 'success', title: 'Skill unlinked from agent' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to unlink skill from agent', message: err?.response?.data?.error || err.message }); },
  });

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-lg rounded-lg bg-white shadow-xl">
        <div className="flex items-center justify-between border-b border-gray-200 px-6 py-4">
          <h2 className="text-lg font-semibold text-gray-900">Link Agents to "{skill.name}"</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
        </div>
        <div className="max-h-[60vh] overflow-auto p-6 space-y-2">
          {(linkedSkills ?? []).map(({ agent, linked }) => (
            <div key={agent.id} className="flex items-center justify-between rounded-md border border-gray-200 px-4 py-3">
              <div>
                <p className="text-sm font-medium text-gray-900">{agent.name}</p>
                <p className="text-xs text-gray-500">{agent.agent_type}</p>
              </div>
              <button
                onClick={() => linked ? unlinkMut.mutate({ agentId: agent.id }) : linkMut.mutate({ agentId: agent.id })}
                className={`rounded-md px-3 py-1.5 text-xs font-medium ${
                  linked ? 'bg-red-50 text-red-700 hover:bg-red-100' : 'bg-blue-50 text-blue-700 hover:bg-blue-100'
                }`}
              >
                {linked ? 'Unlink' : 'Link'}
              </button>
            </div>
          ))}
          {(linkedSkills ?? []).length === 0 && <p className="text-sm text-gray-500">No agents available</p>}
        </div>
        <div className="border-t border-gray-200 px-6 py-4">
          <button onClick={onClose} className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Close</button>
        </div>
      </div>
    </div>
  );
}

// -- Skill Detail --
// Exported for direct testing (SkillDetailPage wraps it with routing/data
// fetching that would otherwise have to be mocked to exercise this logic).
export function SkillDetail({ skill, onBack }: { skill: Skill; onBack: () => void }) {
  const [showLinkModal, setShowLinkModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const queryClient = useQueryClient();
  const { addToast } = useToast();

  // Always resends the skill's full current data -- see toUpdatePayload's
  // comment. Sending only {enabled: ...} used to both 400 (name/slug are
  // binding:"required" on the backend) and, had it not 400'd, would have
  // blanked out every other field.
  const toggleMut = useMutation({
    mutationFn: () => updateSkill(skill.id, { ...toUpdatePayload(skill), enabled: !skill.enabled }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['skills'] }); addToast({ type: 'success', title: `Skill ${skill.enabled ? 'disabled' : 'enabled'}` }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to update skill', message: err?.response?.data?.error || err.message }); },
  });

  const editMut = useMutation({
    mutationFn: (data: CreateSkillRequest) => updateSkill(skill.id, { ...data, enabled: skill.enabled }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['skills'] });
      queryClient.invalidateQueries({ queryKey: ['skill', skill.id] });
      setShowEditModal(false);
      addToast({ type: 'success', title: 'Skill updated' });
    },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to update skill', message: err?.response?.data?.error || err.message }); },
  });

  return (
    <div className="p-8">
      <button onClick={onBack} className="mb-4 inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-700">
        <ArrowLeft className="h-4 w-4" /> Back to Skills
      </button>
      <div className="flex items-center gap-3">
        <h1 className="text-2xl font-bold text-gray-900">{skill.name}</h1>
        <code className="rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-600">{skill.slug}</code>
        <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${categoryBadgeColor[skill.category]}`}>
          {skill.category}
        </span>
        <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${skill.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'}`}>
          {skill.enabled ? 'Enabled' : 'Disabled'}
        </span>
        <button onClick={() => setShowEditModal(true)}
          className="inline-flex items-center gap-1 rounded-md border border-gray-300 px-3 py-1 text-xs font-medium text-gray-700 hover:bg-gray-50">
          <Pencil className="h-3 w-3" /> Edit
        </button>
        <button onClick={() => toggleMut.mutate()} disabled={toggleMut.isPending}
          className="rounded-md border border-gray-300 px-3 py-1 text-xs font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50">
          {skill.enabled ? 'Disable' : 'Enable'}
        </button>
      </div>
      {skill.description && <p className="mt-2 text-sm text-gray-600">{skill.description}</p>}

      <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-2">
        {skill.prompt_template && (
          <div className="lg:col-span-2 rounded-lg border border-gray-200 bg-white p-5">
            <h3 className="mb-3 text-sm font-semibold text-gray-900">Prompt Template</h3>
            <pre className="overflow-auto rounded-md bg-gray-50 p-4 text-xs text-gray-700 whitespace-pre-wrap">{skill.prompt_template}</pre>
          </div>
        )}

        <div className="rounded-lg border border-gray-200 bg-white p-5">
          <h3 className="mb-3 text-sm font-semibold text-gray-900">Required Resource Types</h3>
          {(skill.required_resource_types ?? []).length > 0 ? (
            <div className="flex flex-wrap gap-1">
              {skill.required_resource_types!.map((rt) => (
                <span key={rt} className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">{rt}</span>
              ))}
            </div>
          ) : <p className="text-sm text-gray-400">None</p>}
        </div>

        <div className="rounded-lg border border-gray-200 bg-white p-5">
          <h3 className="mb-3 text-sm font-semibold text-gray-900">Required Permissions</h3>
          {(skill.required_permissions ?? []).length > 0 ? (
            <div className="flex flex-wrap gap-1">
              {skill.required_permissions!.map((p) => (
                <span key={p} className="rounded-full bg-red-50 px-2 py-0.5 text-xs font-medium text-red-700">{p}</span>
              ))}
            </div>
          ) : <p className="text-sm text-gray-400">None</p>}
        </div>

        <div className="lg:col-span-2 rounded-lg border border-gray-200 bg-white p-5">
          <div className="flex items-center justify-between mb-3">
            <h3 className="flex items-center gap-2 text-sm font-semibold text-gray-900"><Link2 className="h-4 w-4 text-gray-400" /> Linked Agents</h3>
            <button onClick={() => setShowLinkModal(true)}
              className="inline-flex items-center gap-1 text-xs font-medium text-blue-600 hover:underline">
              <Plus className="h-3 w-3" /> Manage Links
            </button>
          </div>
          <p className="text-sm text-gray-500">Click "Manage Links" to link or unlink agents from this skill.</p>
        </div>
      </div>

      {showLinkModal && <LinkAgentsModal skill={skill} onClose={() => setShowLinkModal(false)} />}
      {showEditModal && (
        <SkillFormModal
          skill={skill}
          onSubmit={(data) => editMut.mutate(data)}
          onClose={() => setShowEditModal(false)}
          isPending={editMut.isPending}
        />
      )}
    </div>
  );
}

// -- Skill Detail Route --
// Gives a skill a real, bookmarkable /skills/:id URL -- previously the
// detail view only existed as local component state on the list page, so
// a refresh (or anything else that remounted SkillsPage) silently dropped
// back to the list with no indication why.
export function SkillDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const { data: skill, isLoading, isError } = useQuery({
    queryKey: ['skill', id],
    queryFn: () => getSkill(id!),
    enabled: !!id,
  });

  if (isLoading) {
    return <div className="p-8 text-sm text-gray-500">Loading...</div>;
  }
  if (isError || !skill) {
    return (
      <div className="p-8">
        <button onClick={() => navigate('/skills')} className="mb-4 inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-700">
          <ArrowLeft className="h-4 w-4" /> Back to Skills
        </button>
        <p className="text-sm text-gray-500">Skill not found.</p>
      </div>
    );
  }

  return <SkillDetail skill={skill} onBack={() => navigate('/skills')} />;
}

// -- Main Page --
export function SkillsPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [showModal, setShowModal] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ['skills'],
    queryFn: () => listSkills(),
  });

  const createMut = useMutation({
    mutationFn: createSkill,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['skills'] });
      setShowModal(false);
      addToast({ type: 'success', title: 'Skill created successfully' });
    },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to create skill', message: err?.response?.data?.error || err.message }); },
  });

  const deleteMut = useMutation({
    mutationFn: deleteSkill,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['skills'] }); addToast({ type: 'success', title: 'Skill deleted' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to delete skill', message: err?.response?.data?.error || err.message }); },
  });

  const skills = data?.data ?? [];

  return (
    <div className="p-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Skills Registry</h1>
          <p className="mt-1 text-sm text-gray-500">Manage reusable skills that can be linked to agents</p>
        </div>
        <button
          onClick={() => setShowModal(true)}
          className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
        >
          <Plus className="h-4 w-4" /> Create Skill
        </button>
      </div>

      <div className="mt-6">
        {isLoading ? (
          <p className="text-sm text-gray-500">Loading...</p>
        ) : skills.length === 0 ? (
          <div className="rounded-lg border border-dashed border-gray-300 p-12 text-center">
            <Puzzle className="mx-auto h-12 w-12 text-gray-300" />
            <p className="mt-3 text-sm text-gray-500">No skills registered yet</p>
            <button onClick={() => setShowModal(true)}
              className="mt-3 inline-flex items-center gap-1 text-sm font-medium text-blue-600 hover:underline">
              <Plus className="h-4 w-4" /> Create your first skill
            </button>
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
            {skills.map((skill) => (
              <SkillCard
                key={skill.id}
                skill={skill}
                onDelete={() => deleteMut.mutate(skill.id)}
                onClick={() => navigate(`/skills/${skill.id}`)}
              />
            ))}
          </div>
        )}
      </div>

      {showModal && (
        <SkillFormModal
          onSubmit={(data) => createMut.mutate(data)}
          onClose={() => setShowModal(false)}
          isPending={createMut.isPending}
        />
      )}
    </div>
  );
}
