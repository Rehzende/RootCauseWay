import { useState } from 'react';
import { Shield, Users, Clock, Server, Pencil, Check } from 'lucide-react';
import type { IncidentRCI, AnalysisStatus } from '@/types/api';
import { PermissionButton } from '@/components/PermissionButton';

interface RCIPanelProps {
  rci: IncidentRCI | null;
  onUpdate?: (data: Partial<IncidentRCI>) => void;
}

const statusStyle: Record<AnalysisStatus, string> = {
  draft: 'bg-gray-100 text-gray-700',
  in_progress: 'bg-blue-100 text-blue-700',
  completed: 'bg-green-100 text-green-700',
  reviewed: 'bg-purple-100 text-purple-700',
};

export function RCIPanel({ rci, onUpdate }: RCIPanelProps) {
  const [editing, setEditing] = useState(false);
  const [summary, setSummary] = useState(rci?.investigation_summary ?? '');
  const [impact, setImpact] = useState(rci?.impact_assessment ?? '');

  if (!rci) {
    return (
      <div className="flex h-48 items-center justify-center text-sm text-gray-400">
        RCI not generated yet.
      </div>
    );
  }

  const save = () => {
    onUpdate?.({ investigation_summary: summary, impact_assessment: impact });
    setEditing(false);
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-gray-900">Root Cause Investigation</h3>
        <div className="flex items-center gap-2">
          <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${statusStyle[rci.status]}`}>
            {rci.status.replace('_', ' ')}
          </span>
          {onUpdate && (
            <PermissionButton
              resource="incidents" action="write"
              onClick={() => editing ? save() : setEditing(true)}
              className="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
            >
              {editing ? <Check className="h-4 w-4" /> : <Pencil className="h-4 w-4" />}
            </PermissionButton>
          )}
        </div>
      </div>

      {/* Investigation Summary */}
      <div>
        <label className="mb-1 block text-xs font-medium text-gray-500">Investigation Summary</label>
        {editing ? (
          <textarea
            value={summary}
            onChange={(e) => setSummary(e.target.value)}
            className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none"
            rows={4}
          />
        ) : (
          <p className="rounded-lg bg-gray-50 p-3 text-sm text-gray-700">
            {rci.investigation_summary || 'No summary yet.'}
          </p>
        )}
      </div>

      {/* Impact Assessment */}
      <div>
        <label className="mb-1 block text-xs font-medium text-gray-500">Impact Assessment</label>
        {editing ? (
          <textarea
            value={impact}
            onChange={(e) => setImpact(e.target.value)}
            className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none"
            rows={3}
          />
        ) : (
          <p className="rounded-lg bg-amber-50 p-3 text-sm text-amber-900">
            {rci.impact_assessment || 'No assessment yet.'}
          </p>
        )}
      </div>

      {/* Stats grid */}
      <div className="grid grid-cols-2 gap-3">
        <div className="rounded-lg border border-gray-200 p-3">
          <div className="mb-1 flex items-center gap-1.5 text-xs text-gray-500">
            <Server className="h-3.5 w-3.5" /> Affected Services
          </div>
          <div className="flex flex-wrap gap-1">
            {(rci.affected_services ?? []).length > 0 ? (rci.affected_services ?? []).map((s) => (
              <span key={s} className="rounded bg-red-50 px-2 py-0.5 text-xs text-red-700">{s}</span>
            )) : <span className="text-xs text-gray-400">None</span>}
          </div>
        </div>
        <div className="rounded-lg border border-gray-200 p-3">
          <div className="mb-1 flex items-center gap-1.5 text-xs text-gray-500">
            <Users className="h-3.5 w-3.5" /> Affected Users
          </div>
          <p className="text-lg font-semibold text-gray-900">
            {rci.affected_users_estimate != null ? rci.affected_users_estimate.toLocaleString() : '--'}
          </p>
        </div>
        <div className="rounded-lg border border-gray-200 p-3">
          <div className="mb-1 flex items-center gap-1.5 text-xs text-gray-500">
            <Shield className="h-3.5 w-3.5" /> Detection Method
          </div>
          <p className="text-sm text-gray-700">{rci.detection_method || '--'}</p>
        </div>
        <div className="rounded-lg border border-gray-200 p-3">
          <div className="mb-1 flex items-center gap-1.5 text-xs text-gray-500">
            <Clock className="h-3.5 w-3.5" /> Detection Time
          </div>
          <p className="text-sm text-gray-700">
            {rci.detection_time ? new Date(rci.detection_time).toLocaleString() : '--'}
          </p>
        </div>
      </div>
    </div>
  );
}
