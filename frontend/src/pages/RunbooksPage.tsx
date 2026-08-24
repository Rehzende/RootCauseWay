import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { BookOpen, Plus, Zap, Hand, X } from 'lucide-react';
import { useToast } from '@/components/Toast';
import { listRunbooks, createRunbook, updateRunbook, deleteRunbook, listSoftware } from '@/services/api';
import type { Runbook } from '@/types/api';

export function RunbooksPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState('');
  const [newSlug, setNewSlug] = useState('');
  const [newDesc, setNewDesc] = useState('');
  const [newSoftwareId, setNewSoftwareId] = useState('');
  const [newAutoTrigger, setNewAutoTrigger] = useState(false);

  const { data: runbooksData, isLoading } = useQuery({
    queryKey: ['runbooks'],
    queryFn: () => listRunbooks(),
  });

  const { data: softwareData } = useQuery({
    queryKey: ['software-list'],
    queryFn: () => listSoftware(1, 100),
  });

  const createMut = useMutation({
    mutationFn: (data: Partial<Runbook>) => createRunbook(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['runbooks'] });
      setShowCreate(false);
      setNewName('');
      setNewSlug('');
      setNewDesc('');
      setNewSoftwareId('');
      setNewAutoTrigger(false);
      addToast({ type: 'success', title: 'Runbook created successfully' });
    },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to create runbook', message: err?.response?.data?.error || err.message }); },
  });

  const toggleMut = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) => updateRunbook(id, { enabled }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['runbooks'] }); addToast({ type: 'success', title: 'Runbook updated' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to update runbook', message: err?.response?.data?.error || err.message }); },
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => deleteRunbook(id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['runbooks'] }); addToast({ type: 'success', title: 'Runbook deleted' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to delete runbook', message: err?.response?.data?.error || err.message }); },
  });

  const runbooks = runbooksData ?? [];
  const software = softwareData?.data ?? [];

  return (
    <div className="p-8">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Runbooks</h1>
          <p className="mt-1 text-sm text-gray-500">Automated and manual response procedures</p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
        >
          <Plus className="h-4 w-4" /> New Runbook
        </button>
      </div>

      {isLoading ? (
        <div className="flex h-40 items-center justify-center">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-blue-500 border-t-transparent" />
        </div>
      ) : runbooks.length === 0 ? (
        <div className="rounded-lg border border-gray-200 bg-white p-12 text-center">
          <BookOpen className="mx-auto h-12 w-12 text-gray-300" />
          <p className="mt-3 text-sm text-gray-500">No runbooks yet. Create one to get started.</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
          {runbooks.map((rb) => {
            const sw = software.find((s) => s.id === rb.software_id);
            return (
              <div
                key={rb.id}
                className="group cursor-pointer rounded-lg border border-gray-200 bg-white p-5 transition hover:border-blue-300 hover:shadow-sm"
                onClick={() => navigate(`/runbooks/${rb.id}`)}
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1 min-w-0">
                    <h3 className="truncate text-sm font-semibold text-gray-900">{rb.name}</h3>
                    {rb.description && (
                      <p className="mt-1 truncate text-xs text-gray-500">{rb.description}</p>
                    )}
                  </div>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      toggleMut.mutate({ id: rb.id, enabled: !rb.enabled });
                    }}
                    className={`ml-2 flex-shrink-0 rounded-full px-2 py-0.5 text-xs font-medium ${
                      rb.enabled ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'
                    }`}
                  >
                    {rb.enabled ? 'Enabled' : 'Disabled'}
                  </button>
                </div>

                <div className="mt-3 flex flex-wrap items-center gap-2">
                  {sw && (
                    <span className="rounded bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">
                      {sw.name}
                    </span>
                  )}
                  <span className={`inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs font-medium ${
                    rb.auto_trigger ? 'bg-amber-50 text-amber-700' : 'bg-gray-50 text-gray-600'
                  }`}>
                    {rb.auto_trigger ? <><Zap className="h-3 w-3" /> Auto</> : <><Hand className="h-3 w-3" /> Manual</>}
                  </span>
                  {rb.steps && (
                    <span className="text-xs text-gray-400">{rb.steps.length} step{rb.steps.length !== 1 ? 's' : ''}</span>
                  )}
                </div>

                <div className="mt-3 flex items-center justify-end opacity-0 transition group-hover:opacity-100">
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      if (confirm('Delete this runbook?')) deleteMut.mutate(rb.id);
                    }}
                    className="text-xs text-red-500 hover:text-red-700"
                  >
                    Delete
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Create Modal */}
      {showCreate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-full max-w-lg rounded-lg bg-white p-6 shadow-xl">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-semibold text-gray-900">New Runbook</h2>
              <button onClick={() => setShowCreate(false)} className="text-gray-400 hover:text-gray-600">
                <X className="h-5 w-5" />
              </button>
            </div>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                createMut.mutate({
                  name: newName,
                  slug: newSlug || newName.toLowerCase().replace(/\s+/g, '-'),
                  description: newDesc || undefined,
                  software_id: newSoftwareId || undefined,
                  auto_trigger: newAutoTrigger,
                  trigger_conditions: {},
                  enabled: true,
                });
              }}
              className="space-y-4"
            >
              <div>
                <label className="block text-xs font-medium text-gray-700">Name</label>
                <input value={newName} onChange={(e) => setNewName(e.target.value)} required
                  className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-700">Slug</label>
                <input value={newSlug} onChange={(e) => setNewSlug(e.target.value)}
                  placeholder={newName.toLowerCase().replace(/\s+/g, '-')}
                  className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-700">Description</label>
                <textarea value={newDesc} onChange={(e) => setNewDesc(e.target.value)} rows={2}
                  className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-700">Software</label>
                <select value={newSoftwareId} onChange={(e) => setNewSoftwareId(e.target.value)}
                  className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none">
                  <option value="">None</option>
                  {software.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
                </select>
              </div>
              <label className="flex items-center gap-2 text-sm text-gray-700">
                <input type="checkbox" checked={newAutoTrigger} onChange={(e) => setNewAutoTrigger(e.target.checked)}
                  className="rounded border-gray-300" />
                Auto-trigger
              </label>
              <div className="flex justify-end gap-2">
                <button type="button" onClick={() => setShowCreate(false)}
                  className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">
                  Cancel
                </button>
                <button type="submit" disabled={createMut.isPending || !newName.trim()}
                  className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">
                  Create
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
