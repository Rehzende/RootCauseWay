import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useToast } from '@/components/Toast';
import { ShieldAlert, Link2, Clock, Tag, ChevronDown } from 'lucide-react';
import api from '@/services/api';
import type { SoftwareEntry } from '@/types/api';

interface QuarantinedAlert {
  id: string;
  source: string;
  normalized_title: string;
  normalized_severity: string;
  labels: Record<string, string> | string;
  reason: string;
  resolved: boolean;
  created_at: string;
}

const severityColor: Record<string, string> = {
  critical: 'bg-red-100 text-red-700',
  high: 'bg-orange-100 text-orange-700',
  medium: 'bg-yellow-100 text-yellow-700',
  low: 'bg-green-100 text-green-700',
};

export function QuarantinePage() {
  const { addToast } = useToast();
  const qc = useQueryClient();
  const [showResolved, setShowResolved] = useState(false);

  const { data: quarantine } = useQuery({
    queryKey: ['quarantine', showResolved],
    queryFn: () =>
      api.get<{ data: QuarantinedAlert[]; total: number }>(`/quarantine?resolved=${showResolved}&per_page=50`)
        .then((r: { data: { data: QuarantinedAlert[]; total: number } }) => r.data),
    refetchInterval: 10000,
  });

  const { data: softwareList } = useQuery({
    queryKey: ['software-list'],
    queryFn: () =>
      api.get<{ data: SoftwareEntry[] }>('/software?per_page=100').then((r: { data: { data: SoftwareEntry[] } }) => r.data.data),
  });

  const items = quarantine?.data ?? [];
  const total = quarantine?.total ?? 0;

  return (
    <div className="p-8 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 flex items-center gap-2">
            <ShieldAlert className="h-6 w-6 text-amber-500" /> Alert Quarantine
          </h1>
          <p className="mt-1 text-sm text-gray-500">
            Alerts that couldn't be matched to a service. Link them to continue investigation.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-sm text-gray-500">{total} alerts</span>
          <button
            onClick={() => setShowResolved(!showResolved)}
            className={`rounded-md px-3 py-1.5 text-xs font-medium ${
              showResolved ? 'bg-gray-200 text-gray-700' : 'bg-white border border-gray-300 text-gray-600'
            }`}
          >
            {showResolved ? 'Show Pending' : 'Show Resolved'}
          </button>
        </div>
      </div>

      {items.length === 0 ? (
        <div className="rounded-lg border border-dashed border-gray-300 bg-white p-12 text-center">
          <ShieldAlert className="mx-auto h-12 w-12 text-gray-300" />
          <p className="mt-4 text-sm text-gray-500">
            {showResolved ? 'No resolved quarantine alerts.' : 'No alerts in quarantine. All alerts are matched to services.'}
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          {items.map((item) => (
            <QuarantineCard
              key={item.id}
              item={item}
              softwareList={softwareList ?? []}
              onResolved={() => {
                qc.invalidateQueries({ queryKey: ['quarantine'] });
                addToast({ title: 'Success', message: 'Alert linked and incident created', type: 'success' });
              }}
              onError={(msg: string) => addToast({ title: 'Error', message: msg, type: 'error' })}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function QuarantineCard({
  item,
  softwareList,
  onResolved,
  onError,
}: {
  item: QuarantinedAlert;
  softwareList: SoftwareEntry[];
  onResolved: () => void;
  onError: (msg: string) => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const [selectedSoftware, setSelectedSoftware] = useState('');

  const resolveMut = useMutation({
    mutationFn: (softwareId: string) =>
      api.post(`/quarantine/${item.id}/resolve`, { software_id: softwareId }),
    onSuccess: () => onResolved(),
    onError: () => onError('Failed to resolve quarantine alert'),
  });

  const labels: Record<string, string> = typeof item.labels === 'string'
    ? (() => { try { return JSON.parse(item.labels); } catch { return {}; } })()
    : (item.labels ?? {});

  return (
    <div className="rounded-lg border border-amber-200 bg-white p-4">
      <div
        className="flex items-center justify-between cursor-pointer"
        onClick={() => setExpanded(!expanded)}
      >
        <div className="flex items-center gap-3">
          <div className="flex h-8 w-8 items-center justify-center rounded-full bg-amber-100">
            <ShieldAlert className="h-4 w-4 text-amber-600" />
          </div>
          <div>
            <p className="text-sm font-medium text-gray-900">
              {item.normalized_title || 'Unnamed Alert'}
            </p>
            <div className="flex items-center gap-2 mt-0.5">
              <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${severityColor[item.normalized_severity] ?? 'bg-gray-100 text-gray-600'}`}>
                {item.normalized_severity}
              </span>
              <span className="text-xs text-gray-400">{item.source}</span>
              <span className="text-xs text-gray-400 flex items-center gap-1">
                <Clock className="h-3 w-3" />
                {new Date(item.created_at).toLocaleString()}
              </span>
            </div>
          </div>
        </div>
        <ChevronDown className={`h-4 w-4 text-gray-400 transition ${expanded ? 'rotate-180' : ''}`} />
      </div>

      {expanded && (
        <div className="mt-4 space-y-3 border-t border-gray-100 pt-3">
          {/* Labels */}
          {Object.keys(labels).length > 0 && (
            <div>
              <p className="text-xs font-medium text-gray-500 flex items-center gap-1">
                <Tag className="h-3 w-3" /> Labels
              </p>
              <div className="mt-1 flex flex-wrap gap-1">
                {Object.entries(labels).map(([k, v]) => (
                  <span key={k} className="rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-700">
                    {k}={v}
                  </span>
                ))}
              </div>
            </div>
          )}

          {/* Link to software */}
          {!item.resolved && (
            <div className="flex items-end gap-2">
              <div className="flex-1">
                <label className="block text-xs font-medium text-gray-500 mb-1">
                  <Link2 className="inline h-3 w-3 mr-1" />
                  Link to Service
                </label>
                <select
                  value={selectedSoftware}
                  onChange={(e) => setSelectedSoftware(e.target.value)}
                  className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none"
                >
                  <option value="">Select a service...</option>
                  {softwareList.map((sw) => (
                    <option key={sw.id} value={sw.id}>
                      {sw.name} ({sw.slug})
                    </option>
                  ))}
                </select>
              </div>
              <button
                onClick={() => {
                  if (!selectedSoftware) return;
                  resolveMut.mutate(selectedSoftware);
                }}
                disabled={!selectedSoftware || resolveMut.isPending}
                className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
              >
                {resolveMut.isPending ? 'Linking...' : 'Link & Create Incident'}
              </button>
            </div>
          )}

          {item.resolved && (
            <p className="text-xs text-green-600 font-medium">
              Resolved — linked to software and incident created.
            </p>
          )}
        </div>
      )}
    </div>
  );
}
