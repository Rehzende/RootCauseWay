import { useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  ArrowLeft, Play, Plus, ChevronUp, ChevronDown, Trash2, X,
  CheckCircle2, XCircle, Clock, Loader2, Circle,
} from 'lucide-react';
import { useToast } from '@/components/Toast';
import {
  getRunbook, listRunbookSteps, createRunbookStep, deleteRunbookStep, reorderRunbookSteps,
  executeRunbook, listRunbookExecutions, getRunbookExecution, completeExecutionStep,
} from '@/services/api';
import type { RunbookStep, RunbookExecution, RunbookStepType } from '@/types/api';

const stepTypeStyles: Record<RunbookStepType, { bg: string; text: string; label: string }> = {
  manual: { bg: 'bg-blue-100', text: 'text-blue-700', label: 'Manual' },
  automated: { bg: 'bg-green-100', text: 'text-green-700', label: 'Automated' },
  approval: { bg: 'bg-amber-100', text: 'text-amber-700', label: 'Approval' },
  notification: { bg: 'bg-purple-100', text: 'text-purple-700', label: 'Notification' },
  condition: { bg: 'bg-gray-100', text: 'text-gray-700', label: 'Condition' },
};

const stepTypeColors: Record<RunbookStepType, string> = {
  manual: '#3b82f6',
  automated: '#22c55e',
  approval: '#f59e0b',
  notification: '#a855f7',
  condition: '#6b7280',
};

function StepPipeline({ steps, onDelete, onMove }: {
  steps: RunbookStep[];
  onDelete: (id: string) => void;
  onMove: (id: string, dir: 'up' | 'down') => void;
}) {
  if (!steps.length) {
    return <p className="py-8 text-center text-sm text-gray-400">No steps defined yet.</p>;
  }

  return (
    <div className="relative pl-8">
      {/* Connecting line */}
      <div className="absolute left-[15px] top-4 bottom-4 w-0.5 bg-gray-200" />

      {steps.map((step, i) => {
        const style = stepTypeStyles[step.step_type] ?? stepTypeStyles.manual;
        const color = stepTypeColors[step.step_type] ?? '#3b82f6';
        return (
          <div key={step.id} className="relative mb-4 last:mb-0">
            {/* Circle on the line */}
            <div
              className="absolute -left-8 top-4 flex h-[18px] w-[18px] items-center justify-center rounded-full border-2 bg-white"
              style={{ borderColor: color }}
            >
              <span className="text-[9px] font-bold" style={{ color }}>{step.step_order}</span>
            </div>

            <div className="rounded-lg border border-gray-200 bg-white p-4 hover:border-gray-300">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className={`rounded px-2 py-0.5 text-xs font-medium ${style.bg} ${style.text}`}>
                    {style.label}
                  </span>
                  <span className="text-sm font-medium text-gray-900">{step.name}</span>
                </div>
                <div className="flex items-center gap-1">
                  <button
                    onClick={() => onMove(step.id, 'up')}
                    disabled={i === 0}
                    aria-label={`Move ${step.name} up`}
                    className="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 disabled:opacity-30"
                  >
                    <ChevronUp className="h-4 w-4" />
                  </button>
                  <button
                    onClick={() => onMove(step.id, 'down')}
                    disabled={i === steps.length - 1}
                    aria-label={`Move ${step.name} down`}
                    className="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 disabled:opacity-30"
                  >
                    <ChevronDown className="h-4 w-4" />
                  </button>
                  <button
                    onClick={() => onDelete(step.id)}
                    className="rounded p-1 text-gray-400 hover:bg-red-50 hover:text-red-500"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </div>
              {step.description && (
                <p className="mt-1 text-xs text-gray-500">{step.description}</p>
              )}
              <div className="mt-2 flex flex-wrap gap-3 text-xs text-gray-400">
                <span>Timeout: {step.timeout_seconds}s</span>
                <span>On failure: {step.on_failure}</span>
                {step.skill_id && <span>Skill: {step.skill_id}</span>}
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}

function ExecutionView({ execution, onCompleteStep }: { execution: RunbookExecution; onCompleteStep?: (stepId: string) => void }) {
  const stepIcon = (status: string) => {
    switch (status) {
      case 'completed': return <CheckCircle2 className="h-5 w-5 text-green-500" />;
      case 'failed': return <XCircle className="h-5 w-5 text-red-500" />;
      case 'running': return <Loader2 className="h-5 w-5 animate-spin text-blue-500" />;
      case 'pending_action': return <Clock className="h-5 w-5 text-amber-500" />;
      case 'pending_approval': return <Clock className="h-5 w-5 text-orange-500" />;
      default: return <Circle className="h-5 w-5 text-gray-300" />;
    }
  };

  const completedCount = execution.step_results.filter((sr) => sr.status === 'completed').length;
  const totalCount = execution.step_results.length;
  const progressPct = totalCount > 0 ? Math.round((completedCount / totalCount) * 100) : 0;

  const typeStyle = stepTypeStyles as Record<string, { bg: string; text: string; label: string }>;

  return (
    <div className="space-y-3">
      {/* Progress bar */}
      <div>
        <div className="mb-1 flex items-center justify-between text-xs text-gray-500">
          <span>{progressPct}% complete</span>
          <span>{completedCount}/{totalCount} steps</span>
        </div>
        <div className="h-2 w-full rounded-full bg-gray-200">
          <div className="h-2 rounded-full bg-green-500 transition-all" style={{ width: `${progressPct}%` }} />
        </div>
      </div>

      <div className="flex items-center gap-3">
        <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${
          execution.status === 'completed' ? 'bg-green-100 text-green-700' :
          execution.status === 'failed' ? 'bg-red-100 text-red-700' :
          execution.status === 'running' ? 'bg-blue-100 text-blue-700' :
          'bg-gray-100 text-gray-700'
        }`}>
          {execution.status}
        </span>
        {execution.started_at && (
          <span className="text-xs text-gray-400">
            Started: {new Date(execution.started_at).toLocaleString()}
          </span>
        )}
      </div>

      <div className="relative pl-8">
        <div className="absolute left-[15px] top-2 bottom-2 w-0.5 bg-gray-200" />
        {execution.step_results.map((sr, i) => {
          const style = typeStyle[sr.step_type ?? 'manual'] ?? typeStyle.manual;
          const isActionable = (sr.status === 'pending_action' || sr.status === 'pending_approval') && onCompleteStep;
          return (
            <div key={sr.step_id} className="relative mb-3 last:mb-0">
              <div className="absolute -left-8 top-1">
                {stepIcon(sr.status)}
              </div>
              <div className={`rounded border p-3 ${sr.status === 'pending_action' || sr.status === 'running' ? 'border-blue-200 bg-blue-50' : 'border-gray-100 bg-gray-50'}`}>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="text-xs font-medium text-gray-700">{sr.step_name || `Step ${i + 1}`}</span>
                    <span className={`rounded px-1.5 py-0.5 text-[10px] font-medium ${style.bg} ${style.text}`}>{style.label}</span>
                    <span className={`text-xs ${
                      sr.status === 'completed' ? 'text-green-600' :
                      sr.status === 'failed' ? 'text-red-600' :
                      sr.status === 'running' ? 'text-blue-600' :
                      sr.status === 'pending_action' ? 'text-amber-600' :
                      'text-gray-500'
                    }`}>{sr.status.replace(/_/g, ' ')}</span>
                  </div>
                  {isActionable && (
                    <button
                      onClick={() => onCompleteStep(sr.step_id)}
                      className="rounded-md bg-green-600 px-3 py-1 text-xs font-medium text-white hover:bg-green-700"
                    >
                      Mark Complete
                    </button>
                  )}
                </div>
                {sr.completed_at && sr.started_at && (
                  <p className="mt-1 text-[10px] text-gray-400">
                    Duration: {Math.round((new Date(sr.completed_at).getTime() - new Date(sr.started_at).getTime()) / 1000)}s
                  </p>
                )}
                {sr.output && Object.keys(sr.output).length > 0 && (
                  <pre className="mt-1 overflow-auto text-xs text-gray-500">
                    {JSON.stringify(sr.output, null, 2)}
                  </pre>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export function RunbookDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [showAddStep, setShowAddStep] = useState(false);
  const [activeExecId, setActiveExecId] = useState<string | null>(null);
  const [newStep, setNewStep] = useState({ name: '', description: '', step_type: 'manual' as RunbookStepType, timeout_seconds: 300, on_failure: 'stop' });

  const { data: runbook, isLoading } = useQuery({
    queryKey: ['runbook', id],
    queryFn: () => getRunbook(id!),
    enabled: !!id,
  });

  const { data: steps } = useQuery({
    queryKey: ['runbook-steps', id],
    queryFn: () => listRunbookSteps(id!),
    enabled: !!id,
  });

  const { data: executions } = useQuery({
    queryKey: ['runbook-executions', id],
    queryFn: () => listRunbookExecutions(id!),
    enabled: !!id,
  });

  const { data: activeExec } = useQuery({
    queryKey: ['runbook-execution', activeExecId],
    queryFn: () => getRunbookExecution(activeExecId!),
    enabled: !!activeExecId,
    refetchInterval: 3000,
  });

  const addStepMut = useMutation({
    mutationFn: (data: Partial<RunbookStep>) => createRunbookStep(id!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['runbook-steps', id] });
      setShowAddStep(false);
      setNewStep({ name: '', description: '', step_type: 'manual', timeout_seconds: 300, on_failure: 'stop' });
      addToast({ type: 'success', title: 'Step added successfully' });
    },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to add step', message: err?.response?.data?.error || err.message }); },
  });

  const deleteStepMut = useMutation({
    mutationFn: (stepId: string) => deleteRunbookStep(id!, stepId),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['runbook-steps', id] }); addToast({ type: 'success', title: 'Step deleted' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to delete step', message: err?.response?.data?.error || err.message }); },
  });

  const executeMut = useMutation({
    mutationFn: () => executeRunbook(id!),
    onSuccess: (exec) => {
      setActiveExecId(exec.id);
      queryClient.invalidateQueries({ queryKey: ['runbook-executions', id] });
      addToast({ type: 'success', title: 'Runbook execution started' });
    },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to start runbook execution', message: err?.response?.data?.error || err.message }); },
  });

  const completeStepMut = useMutation({
    mutationFn: ({ execId, stepId }: { execId: string; stepId: string }) => completeExecutionStep(execId, stepId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['runbook-execution', activeExecId] });
      addToast({ type: 'success', title: 'Step completed' });
    },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to complete step', message: err?.response?.data?.error || err.message }); },
  });

  const reorderMut = useMutation({
    mutationFn: (stepIds: string[]) => reorderRunbookSteps(id!, stepIds),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['runbook-steps', id] }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to reorder steps', message: err?.response?.data?.error || err.message }); },
  });

  const sortedSteps = [...(steps ?? [])].sort((a, b) => a.step_order - b.step_order);

  const moveStep = (stepId: string, dir: 'up' | 'down') => {
    const idx = sortedSteps.findIndex((s) => s.id === stepId);
    const swapWith = dir === 'up' ? idx - 1 : idx + 1;
    if (idx < 0 || swapWith < 0 || swapWith >= sortedSteps.length) return;
    const reordered = [...sortedSteps];
    [reordered[idx], reordered[swapWith]] = [reordered[swapWith], reordered[idx]];
    reorderMut.mutate(reordered.map((s) => s.id));
  };

  if (isLoading || !runbook) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-blue-500 border-t-transparent" />
      </div>
    );
  }

  return (
    <div className="p-8">
      <button onClick={() => navigate('/runbooks')} className="mb-4 inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-700">
        <ArrowLeft className="h-4 w-4" /> Back to Runbooks
      </button>

      <div className="mb-6 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{runbook.name}</h1>
          {runbook.description && <p className="mt-1 text-sm text-gray-500">{runbook.description}</p>}
          <div className="mt-2 flex items-center gap-2 text-xs text-gray-400">
            <span className="rounded bg-gray-100 px-2 py-0.5">{runbook.slug}</span>
            {runbook.auto_trigger && <span className="rounded bg-amber-50 px-2 py-0.5 text-amber-700">Auto-trigger</span>}
          </div>
        </div>
        <button
          onClick={() => executeMut.mutate()}
          disabled={executeMut.isPending}
          className="inline-flex items-center gap-2 rounded-md bg-green-600 px-4 py-2 text-sm font-medium text-white hover:bg-green-700 disabled:opacity-50"
        >
          <Play className="h-4 w-4" /> Execute
        </button>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        {/* Steps */}
        <div className="lg:col-span-2">
          <div className="rounded-lg border border-gray-200 bg-white p-6">
            <div className="mb-4 flex items-center justify-between">
              <h3 className="text-sm font-semibold text-gray-900">Steps ({sortedSteps.length})</h3>
              <button
                onClick={() => setShowAddStep(true)}
                className="inline-flex items-center gap-1 rounded-md border border-gray-300 px-3 py-1.5 text-xs font-medium text-gray-700 hover:bg-gray-50"
              >
                <Plus className="h-3.5 w-3.5" /> Add Step
              </button>
            </div>

            <StepPipeline
              steps={sortedSteps}
              onDelete={(stepId) => { if (confirm('Delete this step?')) deleteStepMut.mutate(stepId); }}
              onMove={moveStep}
            />
          </div>
        </div>

        {/* Executions sidebar */}
        <div className="space-y-4">
          {activeExec && (
            <div className="rounded-lg border border-blue-200 bg-white p-5">
              <h3 className="mb-3 text-sm font-semibold text-gray-900">Current Execution</h3>
              <ExecutionView
                execution={activeExec}
                onCompleteStep={(stepId) => completeStepMut.mutate({ execId: activeExecId!, stepId })}
              />
            </div>
          )}

          <div className="rounded-lg border border-gray-200 bg-white p-5">
            <h3 className="mb-3 text-sm font-semibold text-gray-900">Recent Executions</h3>
            {(executions ?? []).length === 0 ? (
              <p className="text-xs text-gray-400">No executions yet</p>
            ) : (
              <div className="space-y-2">
                {(executions ?? []).slice(0, 10).map((exec) => (
                  <button
                    key={exec.id}
                    onClick={() => setActiveExecId(exec.id)}
                    className={`flex w-full items-center justify-between rounded border p-2 text-left text-xs transition ${
                      activeExecId === exec.id ? 'border-blue-300 bg-blue-50' : 'border-gray-100 hover:bg-gray-50'
                    }`}
                  >
                    <span className={`rounded-full px-2 py-0.5 font-medium ${
                      exec.status === 'completed' ? 'bg-green-100 text-green-700' :
                      exec.status === 'failed' ? 'bg-red-100 text-red-700' :
                      exec.status === 'running' ? 'bg-blue-100 text-blue-700' :
                      'bg-gray-100 text-gray-600'
                    }`}>{exec.status}</span>
                    <span className="text-gray-400">
                      {exec.started_at ? new Date(exec.started_at).toLocaleString() : '--'}
                    </span>
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Add Step Modal */}
      {showAddStep && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-full max-w-lg rounded-lg bg-white p-6 shadow-xl">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-lg font-semibold text-gray-900">Add Step</h2>
              <button onClick={() => setShowAddStep(false)} className="text-gray-400 hover:text-gray-600">
                <X className="h-5 w-5" />
              </button>
            </div>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                addStepMut.mutate({
                  ...newStep,
                  step_order: sortedSteps.length + 1,
                  config: {},
                });
              }}
              className="space-y-4"
            >
              <div>
                <label className="block text-xs font-medium text-gray-700">Name</label>
                <input value={newStep.name} onChange={(e) => setNewStep({ ...newStep, name: e.target.value })} required
                  className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-700">Description</label>
                <textarea value={newStep.description} onChange={(e) => setNewStep({ ...newStep, description: e.target.value })} rows={2}
                  className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-700">Type</label>
                <select value={newStep.step_type} onChange={(e) => setNewStep({ ...newStep, step_type: e.target.value as RunbookStepType })}
                  className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none">
                  <option value="manual">Manual</option>
                  <option value="automated">Automated</option>
                  <option value="approval">Approval</option>
                  <option value="notification">Notification</option>
                  <option value="condition">Condition</option>
                </select>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-medium text-gray-700">Timeout (seconds)</label>
                  <input type="number" value={newStep.timeout_seconds} onChange={(e) => setNewStep({ ...newStep, timeout_seconds: Number(e.target.value) })}
                    className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
                </div>
                <div>
                  <label className="block text-xs font-medium text-gray-700">On Failure</label>
                  <select value={newStep.on_failure} onChange={(e) => setNewStep({ ...newStep, on_failure: e.target.value })}
                    className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none">
                    <option value="stop">Stop</option>
                    <option value="continue">Continue</option>
                    <option value="retry">Retry</option>
                  </select>
                </div>
              </div>
              <div className="flex justify-end gap-2">
                <button type="button" onClick={() => setShowAddStep(false)}
                  className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
                <button type="submit" disabled={addStepMut.isPending || !newStep.name.trim()}
                  className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">Add Step</button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
