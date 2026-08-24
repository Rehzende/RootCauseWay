import { useState } from 'react';
import { ChevronDown, ChevronUp, RotateCcw, AlertCircle } from 'lucide-react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { AgentRun, AgentRunStatus, IncidentFull } from '@/types/api';
import { useToast } from '@/components/Toast';
import { rerunAgentRun } from '@/services/api';

interface RunsTimelineProps {
  runs: AgentRun[];
  incident: IncidentFull;
}

const statusDotClass: Record<AgentRunStatus, string> = {
  completed: 'bg-green-500',
  running:   'bg-blue-500 animate-pulse',
  failed:    'bg-red-500',
  pending:   'bg-gray-400',
};

const statusLineClass: Record<AgentRunStatus, string> = {
  completed: 'bg-green-500',
  running:   'bg-blue-500',
  failed:    'bg-red-500',
  pending:   'bg-gray-300',
};

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString();
}

function getSummary(run: AgentRun): string {
  const out = run.output_data ?? {};

  if (run.status === 'failed' && run.error_message) {
    return run.error_message;
  }

  const agentType = run.agent_type;

  if (agentType === 'triage') {
    const category = (out as Record<string, unknown>).category ?? (out as Record<string, unknown>).root_cause_category ?? '';
    const confidence = (out as Record<string, unknown>).confidence;
    const parts: string[] = [];
    if (category) parts.push(`Category: ${category}`);
    if (confidence != null) parts.push(`Confidence: ${Math.round(Number(confidence) * 100)}%`);
    return parts.join(' | ') || run.agent_name;
  }

  if (agentType === 'evidence_analysis') {
    const items = (out as Record<string, unknown>).evidence_count ?? (out as Record<string, unknown>).items;
    if (Array.isArray(items)) return `Collected ${items.length} evidence items`;
    if (typeof items === 'number') return `Collected ${items} evidence items`;
    return 'Evidence analysis completed';
  }

  if (agentType === 'hypothesis') {
    const summary = (out as Record<string, unknown>).root_cause_summary as string | undefined;
    const confidence = (out as Record<string, unknown>).confidence;
    const parts: string[] = [];
    if (summary) parts.push(summary.length > 100 ? summary.slice(0, 100) + '...' : summary);
    if (confidence != null) parts.push(`Confidence: ${Math.round(Number(confidence) * 100)}%`);
    return parts.join(' | ') || 'Root cause analysis';
  }

  // Postmortem / custom — try to get a title
  const title = (out as Record<string, unknown>).title as string | undefined;
  if (title) return title.length > 80 ? title.slice(0, 80) + '...' : title;

  return run.agent_name;
}

function JsonBlock({ label, data }: { label: string; data: unknown }) {
  if (data == null || (typeof data === 'object' && Object.keys(data as object).length === 0)) {
    return null;
  }
  return (
    <div>
      <p className="mb-1 text-xs font-medium text-gray-500">{label}</p>
      <pre className="max-h-64 overflow-auto rounded-md bg-gray-50 p-3 text-xs text-gray-700 font-mono leading-relaxed">
        {JSON.stringify(data, null, 2)}
      </pre>
    </div>
  );
}

function RunRow({ run, isLast, incidentId }: { run: AgentRun; isLast: boolean; incidentId: string }) {
  const [expanded, setExpanded] = useState(false);
  const { addToast } = useToast();
  const queryClient = useQueryClient();

  const rerunMut = useMutation({
    mutationFn: () => rerunAgentRun(incidentId, run.id),
    onSuccess: () => {
      addToast({ type: 'success', title: 'Rerun started', message: `Rerunning ${run.agent_name}...` });
      queryClient.invalidateQueries({ queryKey: ['incident-full', incidentId] });
    },
    onError: (err: Error) => {
      addToast({ type: 'error', title: 'Rerun failed', message: err.message || 'Failed to start rerun.' });
    },
  });

  const handleRerun = (e: React.MouseEvent) => {
    e.stopPropagation();
    rerunMut.mutate();
  };

  return (
    <div className="relative flex gap-4">
      {/* Timeline rail */}
      <div className="flex flex-col items-center">
        <div className={`h-3 w-3 rounded-full mt-1.5 flex-shrink-0 ${statusDotClass[run.status]}`} />
        {!isLast && (
          <div className={`w-0.5 flex-1 min-h-[24px] ${statusLineClass[run.status]}`} />
        )}
      </div>

      {/* Card */}
      <div className="flex-1 mb-3">
        <div
          className="rounded-lg border border-gray-200 bg-white shadow-sm hover:shadow-md transition-shadow cursor-pointer"
          onClick={() => setExpanded(!expanded)}
        >
          {/* Header */}
          <div className="flex items-center justify-between px-4 py-3">
            <div className="flex items-center gap-3 min-w-0">
              <span className="text-sm font-semibold text-gray-900 truncate">{run.agent_name}</span>
              {run.duration_ms > 0 && (
                <span className="rounded bg-gray-100 px-2 py-0.5 text-xs font-mono font-bold text-gray-700">
                  {formatDuration(run.duration_ms)}
                </span>
              )}
              {run.model_used && (
                <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-500">
                  {run.model_used}
                </span>
              )}
              {run.tokens_used > 0 && (
                <span className="hidden sm:inline text-xs text-gray-400">
                  {run.tokens_used.toLocaleString()} tok
                </span>
              )}
            </div>
            <div className="flex items-center gap-2 flex-shrink-0">
              {(run.status === 'completed' || run.status === 'failed') && (
                <button
                  onClick={handleRerun}
                  disabled={rerunMut.isPending}
                  className="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 disabled:opacity-50 disabled:cursor-not-allowed"
                  title={rerunMut.isPending ? 'Rerunning...' : 'Rerun this step'}
                >
                  <RotateCcw className={`h-3.5 w-3.5 ${rerunMut.isPending ? 'animate-spin' : ''}`} />
                </button>
              )}
              <span className="text-xs text-gray-400">{formatTime(run.started_at)}</span>
              {expanded ? (
                <ChevronUp className="h-4 w-4 text-gray-400" />
              ) : (
                <ChevronDown className="h-4 w-4 text-gray-400" />
              )}
            </div>
          </div>

          {/* Summary */}
          <div className="px-4 pb-3">
            <p className={`text-sm ${run.status === 'failed' ? 'text-red-600' : 'text-gray-600'}`}>
              {getSummary(run)}
            </p>
          </div>

          {/* Expanded */}
          {expanded && (
            <div className="border-t border-gray-100 px-4 py-4 space-y-4">
              {run.error_message && (
                <div className="flex items-start gap-2 rounded-md bg-red-50 p-3">
                  <AlertCircle className="h-4 w-4 text-red-500 mt-0.5 flex-shrink-0" />
                  <p className="text-sm text-red-700">{run.error_message}</p>
                </div>
              )}
              <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
                <JsonBlock label="Input" data={run.input_data} />
                <JsonBlock label="Output" data={run.output_data} />
              </div>
              <div className="flex flex-wrap gap-3 text-xs text-gray-400">
                <span>ID: {run.id.slice(0, 8)}</span>
                <span>Type: {run.agent_type}</span>
                <span>Status: {run.status}</span>
                {run.completed_at && <span>Completed: {formatTime(run.completed_at)}</span>}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export function RunsTimeline({ runs, incident }: RunsTimelineProps) {
  const sorted = [...runs].sort(
    (a, b) => new Date(a.started_at).getTime() - new Date(b.started_at).getTime(),
  );

  const severityColor: Record<string, string> = {
    critical: 'bg-red-100 text-red-700',
    high: 'bg-orange-100 text-orange-700',
    medium: 'bg-yellow-100 text-yellow-700',
    low: 'bg-blue-100 text-blue-700',
  };

  return (
    <div className="space-y-0">
      {/* Synthetic "Alert Received" entry */}
      <div className="relative flex gap-4">
        <div className="flex flex-col items-center">
          <div className="h-3 w-3 rounded-full mt-1.5 flex-shrink-0 bg-green-500" />
          {sorted.length > 0 && <div className="w-0.5 flex-1 min-h-[24px] bg-green-500" />}
        </div>
        <div className="flex-1 mb-3">
          <div className="rounded-lg border border-gray-200 bg-white shadow-sm px-4 py-3">
            <div className="flex items-center justify-between">
              <span className="text-sm font-semibold text-gray-900">Alert Received</span>
              <span className="text-xs text-gray-400">{formatTime(incident.created_at)}</span>
            </div>
            <div className="mt-1 flex items-center gap-2">
              <p className="text-sm text-gray-600">{incident.title}</p>
              <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${severityColor[incident.severity] ?? 'bg-gray-100 text-gray-600'}`}>
                {incident.severity}
              </span>
            </div>
            {incident.description && (
              <p className="mt-1 text-xs text-gray-400">{incident.description}</p>
            )}
          </div>
        </div>
      </div>

      {/* Agent runs */}
      {sorted.length === 0 ? (
        <div className="flex h-32 items-center justify-center text-sm text-gray-400">
          No agent runs yet.
        </div>
      ) : (
        sorted.map((run, i) => (
          <RunRow key={run.id} run={run} isLast={i === sorted.length - 1} incidentId={incident.id} />
        ))
      )}
    </div>
  );
}
