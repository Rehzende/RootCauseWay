import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Gauge, Plus, Trash2, X } from 'lucide-react';
import { listSoftware } from '@/services/api';
import {
  useSLODefinitions,
  useSLOStatus,
  useCreateSLODefinition,
  useUpdateSLODefinition,
  useDeleteSLODefinition,
} from '@/hooks/useSLO';
import { PermissionGate } from '@/components/PermissionGate';
import { SLOStatusBadge } from '@/components/SLOStatusBadge';
import { EmptyState } from '@/components/EmptyState';
import { SkeletonTable } from '@/components/Skeleton';
import type { SLODefinition, SLOType, CreateSLODefinitionRequest } from '@/types/api';

const sloTypes: SLOType[] = ['availability', 'latency', 'error_rate'];

function emptyForm(): CreateSLODefinitionRequest {
  return { software_id: '', name: '', slo_type: 'availability', target_percentage: 99.9, measurement_window_days: 30 };
}

function SLOModal({
  slo,
  softwareOptions,
  onClose,
}: {
  slo?: SLODefinition;
  softwareOptions: { id: string; name: string }[];
  onClose: () => void;
}) {
  const isEdit = !!slo;
  const [form, setForm] = useState<CreateSLODefinitionRequest>(
    slo
      ? {
          software_id: slo.software_id,
          name: slo.name,
          slo_type: slo.slo_type,
          target_percentage: slo.target_percentage,
          measurement_window_days: slo.measurement_window_days,
        }
      : emptyForm(),
  );

  const createMut = useCreateSLODefinition();
  const updateMut = useUpdateSLODefinition();
  const isPending = createMut.isPending || updateMut.isPending;

  const handleSubmit = () => {
    if (isEdit) {
      updateMut.mutate(
        {
          id: slo!.id,
          data: {
            name: form.name,
            slo_type: form.slo_type,
            target_percentage: form.target_percentage,
            measurement_window_days: form.measurement_window_days,
          },
        },
        { onSuccess: onClose },
      );
    } else {
      createMut.mutate(form, { onSuccess: onClose });
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-md rounded-lg bg-white p-6 shadow-xl">
        <div className="mb-4 flex items-center justify-between">
          <h3 className="text-lg font-semibold text-gray-900">{isEdit ? 'Edit' : 'Create'} SLO</h3>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
        </div>
        <form onSubmit={(e) => { e.preventDefault(); handleSubmit(); }} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700">Software</label>
            <select
              value={form.software_id}
              onChange={(e) => setForm({ ...form, software_id: e.target.value })}
              required
              disabled={isEdit}
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm disabled:bg-gray-50"
            >
              <option value="">Select software...</option>
              {softwareOptions.map((s) => (
                <option key={s.id} value={s.id}>{s.name}</option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Name</label>
            <input
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              required
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">SLO Type</label>
            <select
              value={form.slo_type}
              onChange={(e) => setForm({ ...form, slo_type: e.target.value as SLOType })}
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
            >
              {sloTypes.map((t) => (
                <option key={t} value={t}>{t.replace('_', ' ')}</option>
              ))}
            </select>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700">Target %</label>
              <input
                type="number"
                step="0.01"
                min="0"
                max="100"
                value={form.target_percentage}
                onChange={(e) => setForm({ ...form, target_percentage: parseFloat(e.target.value) || 0 })}
                required
                className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700">Window (days)</label>
              <input
                type="number"
                min="1"
                value={form.measurement_window_days ?? 30}
                onChange={(e) => setForm({ ...form, measurement_window_days: parseInt(e.target.value) || 30 })}
                className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
              />
            </div>
          </div>
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

function SLORow({
  slo,
  softwareName,
  onEdit,
}: {
  slo: SLODefinition;
  softwareName: string;
  onEdit: () => void;
}) {
  const { data: status, isLoading } = useSLOStatus(slo.id);
  const deleteMut = useDeleteSLODefinition();

  return (
    <tr data-testid={`slo-row-${slo.id}`}>
      <td className="whitespace-nowrap px-6 py-4 text-sm font-medium text-gray-900">{slo.name}</td>
      <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-600">{softwareName}</td>
      <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-600">{slo.slo_type.replace('_', ' ')}</td>
      <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-600">{slo.target_percentage}%</td>
      <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-600">
        {isLoading ? '...' : status ? `${status.current_percentage.toFixed(2)}%` : '--'}
      </td>
      <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-600">
        {isLoading ? '...' : status ? `${status.error_budget_remaining_percentage.toFixed(1)}%` : '--'}
      </td>
      <td className="whitespace-nowrap px-6 py-4">
        {isLoading ? (
          <span className="text-xs text-gray-400">Loading...</span>
        ) : status ? (
          <SLOStatusBadge status={status.status} />
        ) : (
          <span className="text-xs text-gray-400">--</span>
        )}
      </td>
      <td className="whitespace-nowrap px-6 py-4 text-right">
        <div className="flex items-center justify-end gap-2">
          <PermissionGate resource="software" action="write">
            <button onClick={onEdit} className="text-sm text-blue-600 hover:text-blue-800">Edit</button>
          </PermissionGate>
          <PermissionGate resource="software" action="write">
            <button
              onClick={() => { if (confirm('Delete this SLO definition?')) deleteMut.mutate(slo.id); }}
              className="text-gray-400 hover:text-red-600"
            >
              <Trash2 className="h-4 w-4" />
            </button>
          </PermissionGate>
        </div>
      </td>
    </tr>
  );
}

export function SLODashboardPage() {
  const [showModal, setShowModal] = useState(false);
  const [editingSLO, setEditingSLO] = useState<SLODefinition | undefined>(undefined);

  const { data: slos, isLoading } = useSLODefinitions();
  const { data: softwareData } = useQuery({ queryKey: ['software-list-all'], queryFn: () => listSoftware(1, 100) });

  const softwareOptions = (softwareData?.data ?? []).map((s) => ({ id: s.id, name: s.name }));
  const softwareNameById = (id: string) => softwareOptions.find((s) => s.id === id)?.name ?? id.slice(0, 8);

  return (
    <div className="p-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold text-gray-900">
            <Gauge className="h-6 w-6 text-blue-500" /> SLO Dashboard
          </h1>
          <p className="mt-1 text-sm text-gray-500">
            Service Level Objectives and error-budget tracking per software entry
          </p>
        </div>
        <PermissionGate resource="software" action="write">
          <button
            onClick={() => { setEditingSLO(undefined); setShowModal(true); }}
            className="flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
          >
            <Plus className="h-4 w-4" /> Create SLO
          </button>
        </PermissionGate>
      </div>

      <div className="mt-6">
        {isLoading ? (
          <SkeletonTable rows={4} cols={7} />
        ) : (slos ?? []).length === 0 ? (
          <EmptyState
            icon={<Gauge className="h-7 w-7 text-gray-400" />}
            title="No SLOs configured"
            description="Create a Service Level Objective to start tracking error budgets for your software."
          />
        ) : (
          <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white shadow-sm">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Name</th>
                  <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Software</th>
                  <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Type</th>
                  <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Target</th>
                  <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Current</th>
                  <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Budget Remaining</th>
                  <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Status</th>
                  <th className="px-6 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {(slos ?? []).map((slo) => (
                  <SLORow
                    key={slo.id}
                    slo={slo}
                    softwareName={softwareNameById(slo.software_id)}
                    onEdit={() => { setEditingSLO(slo); setShowModal(true); }}
                  />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {showModal && (
        <SLOModal
          slo={editingSLO}
          softwareOptions={softwareOptions}
          onClose={() => { setShowModal(false); setEditingSLO(undefined); }}
        />
      )}
    </div>
  );
}
