import { useState } from 'react';
import { Pencil, Check } from 'lucide-react';
import type { IncidentRCA, AnalysisStatus } from '@/types/api';
import { ConfidenceMeter } from './ConfidenceMeter';
import { FiveWhys } from './FiveWhys';

interface RCAPanelProps {
  rca: IncidentRCA | null;
  onUpdate?: (data: Partial<IncidentRCA>) => void;
}

const statusStyle: Record<AnalysisStatus, string> = {
  draft: 'bg-gray-100 text-gray-700',
  in_progress: 'bg-blue-100 text-blue-700',
  completed: 'bg-green-100 text-green-700',
  reviewed: 'bg-purple-100 text-purple-700',
};

export function RCAPanel({ rca, onUpdate }: RCAPanelProps) {
  const [editing, setEditing] = useState(false);
  const [summary, setSummary] = useState(rca?.root_cause_summary ?? '');

  if (!rca) {
    return (
      <div className="flex h-48 items-center justify-center text-sm text-gray-400">
        RCA not generated yet.
      </div>
    );
  }

  const save = () => {
    onUpdate?.({ root_cause_summary: summary });
    setEditing(false);
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-gray-900">Root Cause Analysis</h3>
        <div className="flex items-center gap-2">
          <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${statusStyle[rca.status]}`}>
            {rca.status.replace('_', ' ')}
          </span>
          {onUpdate && (
            <button
              onClick={() => editing ? save() : setEditing(true)}
              className="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
            >
              {editing ? <Check className="h-4 w-4" /> : <Pencil className="h-4 w-4" />}
            </button>
          )}
        </div>
      </div>

      {/* Root Cause Summary */}
      <div>
        <label className="mb-1 block text-xs font-medium text-gray-500">Root Cause Summary</label>
        {editing ? (
          <textarea
            value={summary}
            onChange={(e) => setSummary(e.target.value)}
            className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none"
            rows={4}
          />
        ) : (
          <p className="rounded-lg bg-red-50 p-3 text-sm text-red-900">
            {rca.root_cause_summary || 'Not determined yet.'}
          </p>
        )}
      </div>

      {/* Category */}
      <div>
        <label className="mb-1 block text-xs font-medium text-gray-500">Category</label>
        <span className="inline-block rounded bg-gray-100 px-3 py-1 text-sm font-medium text-gray-700">
          {rca.root_cause_category || 'Uncategorized'}
        </span>
      </div>

      {/* Confidence */}
      <div>
        <label className="mb-1 block text-xs font-medium text-gray-500">Confidence</label>
        <ConfidenceMeter value={rca.confidence ?? 0} />
      </div>

      {/* Contributing Factors */}
      <div>
        <label className="mb-1 block text-xs font-medium text-gray-500">Contributing Factors</label>
        {(rca.contributing_factors ?? []).length > 0 ? (
          <ul className="space-y-1">
            {(rca.contributing_factors ?? []).map((f, i) => (
              <li key={i} className="flex items-start gap-2 text-sm text-gray-700">
                <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-amber-400" />
                {f}
              </li>
            ))}
          </ul>
        ) : (
          <p className="text-sm text-gray-400">None identified.</p>
        )}
      </div>

      {/* 5 Whys */}
      <div>
        <label className="mb-2 block text-xs font-medium text-gray-500">5 Whys Analysis</label>
        <FiveWhys whys={rca.five_whys ?? []} />
      </div>
    </div>
  );
}
