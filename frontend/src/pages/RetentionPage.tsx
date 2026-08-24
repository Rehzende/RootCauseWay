import { useState } from 'react';
import { Archive, Plus, Trash2, X, PlayCircle } from 'lucide-react';
import {
  useRetentionPolicies,
  useCreateRetentionPolicy,
  useUpdateRetentionPolicy,
  useDeleteRetentionPolicy,
  useRunRetentionSweep,
} from '@/hooks/useRetention';
import { PermissionGate } from '@/components/PermissionGate';
import { EmptyState } from '@/components/EmptyState';
import { SkeletonTable } from '@/components/Skeleton';
import type { RetentionPolicy, RetentionResourceType, RetentionActionType, RetentionSweepSummary } from '@/types/api';

const resourceTypes: RetentionResourceType[] = ['evidence', 'incidents', 'agent_runs'];
const actionTypes: RetentionActionType[] = ['archive', 'delete'];

function PolicyModal({ policy, onClose }: { policy?: RetentionPolicy; onClose: () => void }) {
  const isEdit = !!policy;
  const [resourceType, setResourceType] = useState<RetentionResourceType>(policy?.resource_type ?? 'evidence');
  const [retentionDays, setRetentionDays] = useState(policy?.retention_days ?? 90);
  const [action, setAction] = useState<RetentionActionType>(policy?.action ?? 'archive');
  const [enabled, setEnabled] = useState(policy?.enabled ?? true);

  const createMut = useCreateRetentionPolicy();
  const updateMut = useUpdateRetentionPolicy();
  const isPending = createMut.isPending || updateMut.isPending;

  const handleSubmit = () => {
    if (isEdit) {
      updateMut.mutate(
        { id: policy!.id, data: { retention_days: retentionDays, action, enabled } },
        { onSuccess: onClose },
      );
    } else {
      createMut.mutate(
        { resource_type: resourceType, retention_days: retentionDays, action, enabled },
        { onSuccess: onClose },
      );
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-md rounded-lg bg-white p-6 shadow-xl">
        <div className="mb-4 flex items-center justify-between">
          <h3 className="text-lg font-semibold text-gray-900">{isEdit ? 'Edit' : 'Create'} Retention Policy</h3>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
        </div>
        <form onSubmit={(e) => { e.preventDefault(); handleSubmit(); }} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700">Resource Type</label>
            <select
              value={resourceType}
              onChange={(e) => setResourceType(e.target.value as RetentionResourceType)}
              disabled={isEdit}
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm disabled:bg-gray-50"
            >
              {resourceTypes.map((t) => (
                <option key={t} value={t}>{t.replace('_', ' ')}</option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Retention Days</label>
            <input
              type="number"
              min="1"
              value={retentionDays}
              onChange={(e) => setRetentionDays(parseInt(e.target.value) || 1)}
              required
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Action</label>
            <select
              value={action}
              onChange={(e) => setAction(e.target.value as RetentionActionType)}
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
            >
              {actionTypes.map((a) => (
                <option key={a} value={a}>{a}</option>
              ))}
            </select>
          </div>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} className="rounded border-gray-300" />
            Enabled
          </label>
          <div className="flex justify-end gap-2">
            <button type="button" onClick={onClose} className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
            <button type="submit" disabled={isPending} className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">
              {isPending ? 'Saving...' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

function SweepSummaryModal({ summary, onClose }: { summary: RetentionSweepSummary; onClose: () => void }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-lg rounded-lg bg-white p-6 shadow-xl">
        <div className="mb-4 flex items-center justify-between">
          <h3 className="text-lg font-semibold text-gray-900">Sweep Results</h3>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
        </div>
        <p className="mb-3 text-xs text-gray-500">Started at {new Date(summary.started_at).toLocaleString()}</p>
        {summary.results.length === 0 ? (
          <p className="text-sm text-gray-400">No enabled policies matched any records.</p>
        ) : (
          <div className="space-y-3">
            {summary.results.map((r) => (
              <div key={r.policy_id} className="rounded-md border border-gray-200 p-3 text-sm">
                <div className="flex items-center justify-between">
                  <span className="font-medium text-gray-900">{r.resource_type}</span>
                  <span className="rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-600">{r.action}</span>
                </div>
                <div className="mt-2 grid grid-cols-3 gap-2 text-xs text-gray-500">
                  <span>Matched: {r.matched_count}</span>
                  <span>Archived: {r.archived_count}</span>
                  <span>Deleted: {r.deleted_count}</span>
                </div>
                {(r.errors ?? []).length > 0 && (
                  <p className="mt-2 text-xs text-red-600">{r.errors!.length} error(s) occurred</p>
                )}
              </div>
            ))}
          </div>
        )}
        <div className="mt-4 flex justify-end">
          <button onClick={onClose} className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Close</button>
        </div>
      </div>
    </div>
  );
}

export function RetentionPage() {
  const [showModal, setShowModal] = useState(false);
  const [editingPolicy, setEditingPolicy] = useState<RetentionPolicy | undefined>(undefined);
  const [sweepSummary, setSweepSummary] = useState<RetentionSweepSummary | null>(null);

  const { data: policies, isLoading } = useRetentionPolicies();
  const deleteMut = useDeleteRetentionPolicy();
  const sweepMut = useRunRetentionSweep((summary) => setSweepSummary(summary));

  return (
    <div className="p-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold text-gray-900">
            <Archive className="h-6 w-6 text-blue-500" /> Data Retention
          </h1>
          <p className="mt-1 text-sm text-gray-500">
            Configure how long evidence, incidents, and agent runs are kept before archival or deletion
          </p>
        </div>
        <div className="flex items-center gap-2">
          <PermissionGate resource="settings" action="write">
            <button
              onClick={() => sweepMut.mutate()}
              disabled={sweepMut.isPending}
              className="inline-flex items-center gap-2 rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
            >
              <PlayCircle className="h-4 w-4" />
              {sweepMut.isPending ? 'Running Sweep...' : 'Run Sweep Now'}
            </button>
          </PermissionGate>
          <PermissionGate resource="settings" action="write">
            <button
              onClick={() => { setEditingPolicy(undefined); setShowModal(true); }}
              className="flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
            >
              <Plus className="h-4 w-4" /> Add Policy
            </button>
          </PermissionGate>
        </div>
      </div>

      <div className="mt-6">
        {isLoading ? (
          <SkeletonTable rows={3} cols={5} />
        ) : (policies ?? []).length === 0 ? (
          <EmptyState
            icon={<Archive className="h-7 w-7 text-gray-400" />}
            title="No retention policies configured"
            description="Add a policy to automatically archive or delete old evidence, incidents, or agent runs."
          />
        ) : (
          <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white shadow-sm">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Resource Type</th>
                  <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Retention Days</th>
                  <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Action</th>
                  <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Status</th>
                  <th className="px-6 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {(policies ?? []).map((p) => (
                  <tr key={p.id}>
                    <td className="whitespace-nowrap px-6 py-4 text-sm font-medium text-gray-900">{p.resource_type.replace('_', ' ')}</td>
                    <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-600">{p.retention_days} days</td>
                    <td className="whitespace-nowrap px-6 py-4">
                      <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${p.action === 'delete' ? 'bg-red-100 text-red-700' : 'bg-blue-100 text-blue-700'}`}>
                        {p.action}
                      </span>
                    </td>
                    <td className="whitespace-nowrap px-6 py-4">
                      <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${p.enabled ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'}`}>
                        {p.enabled ? 'Enabled' : 'Disabled'}
                      </span>
                    </td>
                    <td className="whitespace-nowrap px-6 py-4 text-right">
                      <div className="flex items-center justify-end gap-2">
                        <PermissionGate resource="settings" action="write">
                          <button onClick={() => { setEditingPolicy(p); setShowModal(true); }} className="text-sm text-blue-600 hover:text-blue-800">Edit</button>
                        </PermissionGate>
                        <PermissionGate resource="settings" action="write">
                          <button
                            onClick={() => { if (confirm('Delete this retention policy?')) deleteMut.mutate(p.id); }}
                            className="text-gray-400 hover:text-red-600"
                          >
                            <Trash2 className="h-4 w-4" />
                          </button>
                        </PermissionGate>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {showModal && (
        <PolicyModal policy={editingPolicy} onClose={() => { setShowModal(false); setEditingPolicy(undefined); }} />
      )}
      {sweepSummary && <SweepSummaryModal summary={sweepSummary} onClose={() => setSweepSummary(null)} />}
    </div>
  );
}
