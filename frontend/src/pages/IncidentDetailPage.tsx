import { useState, useEffect, useMemo } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useWebSocket } from '@/hooks/useWebSocket';
import { useToast } from '@/components/Toast';
import { SkeletonDetailPage } from '@/components/Skeleton';
import {
  ArrowLeft, Clock, User, FileSearch, LayoutGrid, Activity,
  Search, BookOpen, ChevronDown, ThumbsUp, ThumbsDown, X,
  ExternalLink, GitCommit, Layers, Trash2,
} from 'lucide-react';
import {
  getIncidentFull, updateIncident, deleteIncident, addIncidentEvent, updateRCI, updateRCA, updatePostmortem,
  listA2ATasks, submitFeedback, listSimilarIncidents, listChangeEvents, listAlertGroups,
  approveIncidentStage, listUsers,
} from '@/services/api';
import { PresenceIndicator } from '@/components/PresenceIndicator';
import type { PresenceUser } from '@/components/PresenceIndicator';
import { PermissionGate } from '@/components/PermissionGate';
import { PermissionButton } from '@/components/PermissionButton';
import { SeverityBadge } from '@/components/SeverityBadge';
import { StatusBadge } from '@/components/StatusBadge';
import { IncidentTimeline } from '@/components/IncidentTimeline';
import { EvidencePanel } from '@/components/EvidencePanel';
import { EvidenceUpload } from '@/components/EvidenceUpload';
import { RunsTimeline } from '@/components/RunsTimeline';
import { RCIPanel } from '@/components/RCIPanel';
import { RCAPanel } from '@/components/RCAPanel';
import { PostmortemView } from '@/components/PostmortemView';
import { OrchestratorDecisions } from '@/components/OrchestratorDecisions';
import { WarRoomPanel } from '@/components/WarRoomPanel';
import { ApprovalGateBanner } from '@/components/ApprovalGateBanner';
import { formatIncidentCode } from '@/lib/incident';
import { criticalityBadge } from '@/lib/software';
import type { IncidentStatus, AnalysisStatus, A2ATask } from '@/types/api';

const statuses: IncidentStatus[] = ['open', 'investigating', 'mitigated', 'resolved', 'closed'];

type Tab = 'overview' | 'runs' | 'evidence' | 'rci-rca' | 'postmortem';

const tabs: { key: Tab; label: string; icon: typeof LayoutGrid }[] = [
  { key: 'overview', label: 'Overview', icon: LayoutGrid },
  { key: 'runs', label: 'Runs', icon: Activity },
  { key: 'evidence', label: 'Evidence', icon: FileSearch },
  { key: 'rci-rca', label: 'RCI / RCA', icon: Search },
  { key: 'postmortem', label: 'Postmortem', icon: BookOpen },
];

function useDuration(createdAt: string, resolvedAt: string | null) {
  const [now, setNow] = useState(Date.now());

  useEffect(() => {
    if (resolvedAt) return;
    const interval = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(interval);
  }, [resolvedAt]);

  const end = resolvedAt ? new Date(resolvedAt).getTime() : now;
  const diff = end - new Date(createdAt).getTime();
  const h = Math.floor(diff / 3600000);
  const m = Math.floor((diff % 3600000) / 60000);
  const s = Math.floor((diff % 60000) / 1000);
  if (h > 0) return `${h}h ${m}m ${s}s`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

function FeedbackButtons({ incidentId, targetType }: { incidentId: string; targetType: string }) {
  const [showModal, setShowModal] = useState(false);
  const [pendingRating, setPendingRating] = useState<'positive' | 'negative'>('positive');
  const [correction, setCorrection] = useState('');

  const feedbackMut = useMutation({
    mutationFn: (data: { target_type: string; rating: 'positive' | 'negative'; correction?: string }) =>
      submitFeedback(incidentId, data),
    onSuccess: () => {
      setShowModal(false);
      setCorrection('');
    },
  });

  const handleClick = (rating: 'positive' | 'negative') => {
    setPendingRating(rating);
    setShowModal(true);
  };

  return (
    <>
      <div className="flex items-center gap-1">
        <button onClick={() => handleClick('positive')}
          className="rounded p-1 text-gray-400 hover:bg-green-50 hover:text-green-600" title="Good analysis">
          <ThumbsUp className="h-3.5 w-3.5" />
        </button>
        <button onClick={() => handleClick('negative')}
          className="rounded p-1 text-gray-400 hover:bg-red-50 hover:text-red-600" title="Needs improvement">
          <ThumbsDown className="h-3.5 w-3.5" />
        </button>
      </div>
      {showModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-full max-w-sm rounded-lg bg-white p-5 shadow-xl">
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-sm font-semibold text-gray-900">
                {pendingRating === 'positive' ? 'Positive' : 'Negative'} Feedback for {targetType}
              </h3>
              <button onClick={() => setShowModal(false)} className="text-gray-400 hover:text-gray-600"><X className="h-4 w-4" /></button>
            </div>
            <textarea
              placeholder="Optional: Add a correction or comment..."
              value={correction}
              onChange={(e) => setCorrection(e.target.value)}
              rows={3}
              className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none"
            />
            <div className="mt-3 flex justify-end gap-2">
              <button onClick={() => setShowModal(false)}
                className="rounded-md border border-gray-300 px-3 py-1.5 text-xs font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
              <button onClick={() => feedbackMut.mutate({ target_type: targetType, rating: pendingRating, correction: correction || undefined })}
                disabled={feedbackMut.isPending}
                className="rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-700 disabled:opacity-50">Submit</button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}

function ProgressIndicator({ label, status }: { label: string; status?: AnalysisStatus | string | null }) {
  const colors: Record<string, string> = {
    draft: 'bg-gray-200 text-gray-600',
    in_progress: 'bg-blue-100 text-blue-700',
    completed: 'bg-green-100 text-green-700',
    reviewed: 'bg-purple-100 text-purple-700',
    in_review: 'bg-amber-100 text-amber-700',
    published: 'bg-green-100 text-green-700',
  };
  const c = status ? (colors[status] ?? 'bg-gray-100 text-gray-500') : 'bg-gray-100 text-gray-400';

  return (
    <div className="flex items-center justify-between rounded-lg border border-gray-200 px-4 py-3">
      <span className="text-sm font-medium text-gray-700">{label}</span>
      <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${c}`}>
        {status ? status.replace('_', ' ') : 'not started'}
      </span>
    </div>
  );
}

const taskStatusColor: Record<string, string> = {
  pending: 'bg-gray-100 text-gray-700',
  running: 'bg-blue-100 text-blue-700',
  completed: 'bg-green-100 text-green-700',
  failed: 'bg-red-100 text-red-700',
  cancelled: 'bg-gray-100 text-gray-500',
};

function A2ATaskCard({ task }: { task: A2ATask }) {
  const [expanded, setExpanded] = useState(false);
  return (
    <div className="rounded-lg border border-gray-200 p-4">
      <div className="flex items-center justify-between cursor-pointer" onClick={() => setExpanded(!expanded)}>
        <div className="flex items-center gap-2">
          <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${taskStatusColor[task.status] ?? 'bg-gray-100 text-gray-700'}`}>
            {task.status}
          </span>
          <span className="text-sm font-medium text-gray-900">{task.task_type}</span>
          <span className="text-xs text-gray-500">Priority: {task.priority}</span>
        </div>
        <ChevronDown className={`h-4 w-4 text-gray-400 transition ${expanded ? 'rotate-180' : ''}`} />
      </div>
      {expanded && (
        <div className="mt-3 space-y-2 border-t border-gray-100 pt-3">
          {task.orchestrator_reasoning && (
            <div>
              <p className="text-xs font-medium text-gray-500">Orchestrator Reasoning</p>
              <p className="text-sm text-gray-700">{task.orchestrator_reasoning}</p>
            </div>
          )}
          <div>
            <p className="text-xs font-medium text-gray-500">Input Message</p>
            <pre className="mt-1 overflow-auto rounded bg-gray-50 p-2 text-xs text-gray-700">
              {JSON.stringify(task.input_message, null, 2)}
            </pre>
          </div>
          {task.output_artifacts.length > 0 && (
            <div>
              <p className="text-xs font-medium text-gray-500">Output Artifacts</p>
              <pre className="mt-1 overflow-auto rounded bg-gray-50 p-2 text-xs text-gray-700">
                {JSON.stringify(task.output_artifacts, null, 2)}
              </pre>
            </div>
          )}
          {task.error_message && (
            <div>
              <p className="text-xs font-medium text-gray-500 text-red-600">Error</p>
              <p className="text-sm text-red-600">{task.error_message}</p>
            </div>
          )}
          <div className="flex gap-4 text-xs text-gray-400">
            {task.submitted_at && <span>Submitted: {new Date(task.submitted_at).toLocaleString()}</span>}
            {task.started_at && <span>Started: {new Date(task.started_at).toLocaleString()}</span>}
            {task.completed_at && <span>Completed: {new Date(task.completed_at).toLocaleString()}</span>}
          </div>
        </div>
      )}
    </div>
  );
}

function SimilarIncidentsPanel({ incidentId }: { incidentId: string }) {
  const navigate = useNavigate();
  const { data: similar } = useQuery({
    queryKey: ['similar-incidents', incidentId],
    queryFn: () => listSimilarIncidents(incidentId),
  });

  if (!similar || similar.length === 0) return null;

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-5">
      <h3 className="mb-3 text-sm font-semibold text-gray-900">Similar Incidents</h3>
      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        {similar.map((s) => (
          <div key={s.id} className="rounded-lg border border-gray-100 p-4 hover:border-blue-200 transition">
            <div className="flex items-start justify-between">
              <span className="text-sm font-medium text-gray-900">{s.similar_incident_id.slice(0, 8)}...</span>
              <span className={`rounded-full px-2 py-0.5 text-xs font-bold ${
                s.similarity_score >= 0.8 ? 'bg-green-100 text-green-700' :
                s.similarity_score >= 0.5 ? 'bg-amber-100 text-amber-700' :
                'bg-gray-100 text-gray-600'
              }`}>
                {Math.round(s.similarity_score * 100)}%
              </span>
            </div>
            {s.matched_on && Object.keys(s.matched_on).length > 0 && (
              <div className="mt-2 flex flex-wrap gap-1">
                {Object.keys(s.matched_on).map((k) => (
                  <span key={k} className="rounded bg-blue-50 px-1.5 py-0.5 text-xs text-blue-700">{k}</span>
                ))}
              </div>
            )}
            <button
              onClick={() => navigate(`/incidents/${s.similar_incident_id}`)}
              className="mt-2 inline-flex items-center gap-1 text-xs text-blue-600 hover:text-blue-700"
            >
              <ExternalLink className="h-3 w-3" /> View
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}

function RecentChangesPanel({ incidentId, incidentCreatedAt, softwareId }: { incidentId: string; incidentCreatedAt: string; softwareId: string }) {
  const incidentTime = new Date(incidentCreatedAt).getTime();
  const since = new Date(incidentTime - 24 * 60 * 60 * 1000).toISOString();
  const until = new Date(incidentTime + 2 * 60 * 60 * 1000).toISOString();

  const { data: changesData } = useQuery({
    queryKey: ['change-events', softwareId, incidentId],
    queryFn: () => listChangeEvents({ software_id: softwareId, since, until }),
  });

  const changes = changesData?.data ?? [];
  if (changes.length === 0) return null;

  const relativeTime = (occurredAt: string) => {
    const diff = incidentTime - new Date(occurredAt).getTime();
    const absDiff = Math.abs(diff);
    const mins = Math.round(absDiff / 60000);
    const hours = Math.round(absDiff / 3600000);
    const label = diff > 0 ? 'before' : 'after';
    if (mins < 60) return `${mins}min ${label} incident`;
    return `${hours}h ${label} incident`;
  };

  const changeTypeBg: Record<string, string> = {
    deploy: 'bg-green-100 text-green-700',
    config: 'bg-purple-100 text-purple-700',
    rollback: 'bg-red-100 text-red-700',
    migration: 'bg-amber-100 text-amber-700',
  };

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-5">
      <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-900">
        <GitCommit className="h-4 w-4 text-gray-400" /> Recent Changes
      </h3>
      <div className="space-y-3">
        {changes.map((ch) => (
          <div key={ch.id} className="flex items-start gap-3 rounded border border-gray-100 p-3">
            <span className={`flex-shrink-0 rounded px-2 py-0.5 text-xs font-medium ${changeTypeBg[ch.change_type] ?? 'bg-gray-100 text-gray-600'}`}>
              {ch.change_type}
            </span>
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium text-gray-900 truncate">{ch.title}</p>
              <div className="mt-1 flex flex-wrap gap-3 text-xs text-gray-400">
                {ch.author && <span>{ch.author}</span>}
                <span>{relativeTime(ch.occurred_at)}</span>
                {ch.environment && <span className="rounded bg-gray-50 px-1.5">{ch.environment}</span>}
              </div>
            </div>
            {ch.source_url && (
              <a href={ch.source_url} target="_blank" rel="noopener noreferrer" className="text-gray-400 hover:text-blue-500">
                <ExternalLink className="h-4 w-4" />
              </a>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

function CorrelatedAlertsPanel({ incidentId }: { incidentId: string }) {
  const { data: alertGroups } = useQuery({
    queryKey: ['alert-groups', incidentId],
    queryFn: () => listAlertGroups(incidentId),
  });

  if (!alertGroups || alertGroups.length <= 1) return null;

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-5">
      <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-900">
        <Layers className="h-4 w-4 text-gray-400" /> Correlated Alerts
      </h3>
      <p className="text-sm text-gray-600 mb-3">{alertGroups.length} alerts grouped under this incident</p>
      <div className="space-y-2">
        {alertGroups.map((ag) => (
          <div key={ag.id} className="flex items-center justify-between rounded border border-gray-100 px-3 py-2 text-sm">
            <span className="text-gray-700">Alert: {ag.alert_snapshot_id.slice(0, 12)}...</span>
            {ag.correlation_rule_id && (
              <span className="rounded bg-blue-50 px-2 py-0.5 text-xs text-blue-600">
                Rule: {ag.correlation_rule_id.slice(0, 8)}
              </span>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

export function IncidentDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const [comment, setComment] = useState('');
  const [showStatusMenu, setShowStatusMenu] = useState(false);
  const [showAssignMenu, setShowAssignMenu] = useState(false);
  const [pulseSection, setPulseSection] = useState<string | null>(null);

  const wsTopics = useMemo(() => (id ? [`incident:${id}`] : []), [id]);
  const { lastEvent } = useWebSocket(wsTopics);
  const { addToast } = useToast();

  useEffect(() => {
    if (!lastEvent || !id) return;
    const eventType = lastEvent.type;
    if (['incident.updated', 'agent.status', 'rca.completed', 'postmortem.generated'].includes(eventType)) {
      queryClient.invalidateQueries({ queryKey: ['incident-full', id] });
      queryClient.invalidateQueries({ queryKey: ['a2a-tasks', id] });

      if (eventType === 'rca.completed') {
        setPulseSection('rci-rca');
        addToast({ type: 'success', title: 'RCA completed', message: 'Root cause analysis is ready.' });
      } else if (eventType === 'postmortem.generated') {
        setPulseSection('postmortem');
        addToast({ type: 'success', title: 'Postmortem generated', message: 'The postmortem document is ready for review.' });
      } else if (eventType === 'incident.updated') {
        setPulseSection('overview');
      }

      setTimeout(() => setPulseSection(null), 2000);
    }
  }, [lastEvent]); // eslint-disable-line react-hooks/exhaustive-deps

  const { data: incident, isLoading } = useQuery({
    queryKey: ['incident-full', id],
    queryFn: () => getIncidentFull(id!),
    enabled: !!id,
    refetchInterval: 15000,
  });

  const { data: a2aTasks } = useQuery({
    queryKey: ['a2a-tasks', id],
    queryFn: () => listA2ATasks(id!),
    enabled: !!id,
  });

  const { data: usersData } = useQuery({
    queryKey: ['users', 'for-assign'],
    queryFn: () => listUsers({ per_page: 100 }),
  });
  const assignableUsers = usersData?.data ?? [];
  const assignee = assignableUsers.find((u) => u.id === incident?.assignee_id);

  const updateMut = useMutation({
    mutationFn: (data: Parameters<typeof updateIncident>[1]) => updateIncident(id!, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['incident-full', id] }),
  });

  const deleteMut = useMutation({
    mutationFn: () => deleteIncident(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['incidents'] });
      addToast({ type: 'success', title: 'Incident deleted' });
      navigate('/incidents');
    },
    onError: (err: any) => {
      addToast({ type: 'error', title: 'Failed to delete incident', message: err?.response?.data?.error || err.message });
    },
  });

  const addEventMut = useMutation({
    mutationFn: (data: Parameters<typeof addIncidentEvent>[1]) => addIncidentEvent(id!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['incident-full', id] });
      setComment('');
    },
  });

  const rciMut = useMutation({
    mutationFn: (data: Parameters<typeof updateRCI>[1]) => updateRCI(id!, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['incident-full', id] }),
  });

  const rcaMut = useMutation({
    mutationFn: (data: Parameters<typeof updateRCA>[1]) => updateRCA(id!, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['incident-full', id] }),
  });

  const postmortemMut = useMutation({
    mutationFn: (data: Parameters<typeof updatePostmortem>[1]) => updatePostmortem(id!, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['incident-full', id] }),
  });

  const approveStageMut = useMutation({
    mutationFn: () => approveIncidentStage(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['incident-full', id] });
      addToast({ type: 'success', title: 'Stage approved', message: 'The pipeline will now resume.' });
    },
    onError: (err: any) => {
      addToast({
        type: 'error',
        title: 'Failed to approve stage',
        message: err?.response?.data?.error || err.message,
      });
    },
  });

  const duration = useDuration(
    incident?.created_at ?? new Date().toISOString(),
    incident?.resolved_at ?? null,
  );

  // Presence: show current user as viewing. Ready for multi-user via WebSocket.
  const presenceUsers: PresenceUser[] = useMemo(() => [
    { id: 'current-user', name: 'Admin', active: true },
  ], []);

  if (isLoading || !incident) {
    return <SkeletonDetailPage />;
  }

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="border-b border-gray-200 bg-white px-8 py-5">
        <button
          onClick={() => navigate('/incidents')}
          className="mb-3 inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-700"
        >
          <ArrowLeft className="h-4 w-4" /> Back to Incidents
        </button>

        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="flex-1">
            <div className="flex items-center gap-3">
              <h1 className="text-xl font-bold text-gray-900">
                <span className="text-gray-400 font-semibold">{formatIncidentCode(incident.incident_number)}</span> - {incident.title}
              </h1>
              <SeverityBadge severity={incident.severity} />
              <StatusBadge status={incident.status} />
            </div>
            {incident.description && (
              <p className="mt-1.5 text-sm text-gray-600">{incident.description}</p>
            )}
            <div className="mt-3 flex flex-wrap items-center gap-5 text-xs text-gray-500">
              <span className="inline-flex items-center gap-1">
                <Clock className="h-3.5 w-3.5" />
                {new Date(incident.created_at).toLocaleString()}
              </span>
              <span className="inline-flex items-center gap-1 font-mono text-sm font-semibold text-gray-900">
                <Clock className="h-3.5 w-3.5 text-gray-400" />
                {duration}
              </span>
              {incident.assignee_id && (
                <span className="inline-flex items-center gap-1">
                  <User className="h-3.5 w-3.5" /> Assigned to {assignee?.name ?? 'someone no longer in this org'}
                </span>
              )}
            </div>
          </div>

          {/* Presence + Quick actions */}
          <div className="flex items-center gap-4">
            <PresenceIndicator users={presenceUsers} />
          </div>
          <div className="flex items-center gap-2">
            <div className="relative">
              <PermissionButton
                resource="incidents" action="write"
                onClick={() => setShowStatusMenu(!showStatusMenu)}
                className="inline-flex items-center gap-1.5 rounded-md border border-gray-300 bg-white px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
              >
                Change Status <ChevronDown className="h-4 w-4" />
              </PermissionButton>
              {showStatusMenu && (
                <div className="absolute right-0 z-10 mt-1 w-40 rounded-md border border-gray-200 bg-white py-1 shadow-lg">
                  {statuses.map((s) => (
                    <button
                      key={s}
                      onClick={() => { updateMut.mutate({ status: s }); setShowStatusMenu(false); }}
                      className={`block w-full px-4 py-2 text-left text-sm hover:bg-gray-50 ${
                        s === incident.status ? 'font-semibold text-blue-600' : 'text-gray-700'
                      }`}
                    >
                      {s}
                    </button>
                  ))}
                </div>
              )}
            </div>
            <div className="relative">
              <PermissionButton
                resource="incidents" action="write"
                onClick={() => setShowAssignMenu(!showAssignMenu)}
                className="inline-flex items-center gap-1.5 rounded-md border border-gray-300 bg-white px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
              >
                <User className="h-4 w-4" /> {assignee ? assignee.name : 'Assign'} <ChevronDown className="h-4 w-4" />
              </PermissionButton>
              {showAssignMenu && (
                <div className="absolute right-0 z-10 mt-1 w-56 rounded-md border border-gray-200 bg-white py-1 shadow-lg">
                  {incident.assignee_id && (
                    <button
                      onClick={() => { updateMut.mutate({ assignee_id: '' }); setShowAssignMenu(false); }}
                      className="block w-full px-4 py-2 text-left text-sm text-red-600 hover:bg-gray-50"
                    >
                      Unassign
                    </button>
                  )}
                  {assignableUsers.length === 0 && (
                    <p className="px-4 py-2 text-sm text-gray-400">No users available</p>
                  )}
                  {assignableUsers.map((u) => (
                    <button
                      key={u.id}
                      onClick={() => { updateMut.mutate({ assignee_id: u.id }); setShowAssignMenu(false); }}
                      className={`block w-full truncate px-4 py-2 text-left text-sm hover:bg-gray-50 ${
                        u.id === incident.assignee_id ? 'font-semibold text-blue-600' : 'text-gray-700'
                      }`}
                    >
                      {u.name} <span className="text-xs text-gray-400">({u.email})</span>
                    </button>
                  ))}
                </div>
              )}
            </div>
            <PermissionGate resource="incidents" action="delete">
              <button
                onClick={() => {
                  if (confirm(`Delete ${formatIncidentCode(incident.incident_number)} - ${incident.title}? This removes its evidence, timeline, and RCA/postmortem too.`)) {
                    deleteMut.mutate();
                  }
                }}
                className="inline-flex items-center gap-1.5 rounded-md border border-gray-300 bg-white px-3 py-2 text-sm font-medium text-red-600 hover:bg-red-50"
              >
                <Trash2 className="h-4 w-4" />
              </button>
            </PermissionGate>
          </div>
        </div>

        {/* Tabs */}
        <div className="mt-5 flex gap-0 border-b-0">
          {tabs.map(({ key, label, icon: Icon }) => (
            <button
              key={key}
              onClick={() => setActiveTab(key)}
              className={`inline-flex items-center gap-2 border-b-2 px-4 py-2.5 text-sm font-medium transition-colors ${
                activeTab === key
                  ? 'border-blue-600 text-blue-600'
                  : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700'
              }`}
            >
              <Icon className="h-4 w-4" />
              {label}
            </button>
          ))}
        </div>
      </div>

      {/* Tab content */}
      <div className="flex-1 overflow-auto bg-gray-50 p-8">
        {incident.awaiting_approval_stage && (
          <div className="mb-6">
            <ApprovalGateBanner
              awaitingApprovalStage={incident.awaiting_approval_stage}
              onApprove={() => approveStageMut.mutate()}
              isPending={approveStageMut.isPending}
            />
          </div>
        )}

        {activeTab === 'overview' && (
          <div className={`space-y-6 ${pulseSection === 'overview' ? 'animate-pulse-section rounded-lg' : ''}`}>
            {/* Summary cards */}
            <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
              <div className="rounded-lg border border-gray-200 bg-white p-5">
                <p className="text-xs font-medium uppercase text-gray-500">Duration</p>
                <p className="mt-1 text-2xl font-bold text-gray-900">{duration}</p>
              </div>
              <div className="rounded-lg border border-gray-200 bg-white p-5">
                <p className="text-xs font-medium uppercase text-gray-500">Affected Services</p>
                <div className="mt-2 flex flex-wrap gap-1">
                  {incident.rci?.affected_services?.length ? (
                    incident.rci.affected_services.map((s) => (
                      <span key={s} className="rounded bg-red-50 px-2 py-0.5 text-xs font-medium text-red-700">{s}</span>
                    ))
                  ) : (
                    <span className="text-sm text-gray-400">--</span>
                  )}
                </div>
              </div>
              <div className="rounded-lg border border-gray-200 bg-white p-5">
                <p className="text-xs font-medium uppercase text-gray-500">Affected Users</p>
                <p className="mt-1 text-2xl font-bold text-gray-900">
                  {incident.rci?.affected_users_estimate?.toLocaleString() ?? '--'}
                </p>
              </div>
            </div>

            {/* Software Context */}
            {incident.software && (
              <div className="rounded-lg border border-gray-200 bg-white p-5">
                <h3 className="mb-3 text-sm font-semibold text-gray-900">Software Context</h3>
                <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                  <div>
                    <p className="text-xs font-medium uppercase text-gray-500">Service</p>
                    <p className="mt-1 text-sm font-semibold text-gray-900">{incident.software.name}</p>
                    <p className="text-xs text-gray-500">{incident.software.slug}</p>
                  </div>
                  <div>
                    <p className="text-xs font-medium uppercase text-gray-500">Status</p>
                    <span className={`inline-block mt-1 rounded-full px-2 py-0.5 text-xs font-medium ${
                      incident.software.status === 'active' ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-600'
                    }`}>{incident.software.status}</span>
                  </div>
                  {incident.software.criticality && (
                    <div>
                      <p className="text-xs font-medium uppercase text-gray-500">Criticality</p>
                      <span className={`inline-block mt-1 rounded-full px-2 py-0.5 text-xs font-medium ${criticalityBadge[incident.software.criticality] ?? 'bg-gray-100 text-gray-700'}`}>
                        {incident.software.criticality}
                      </span>
                    </div>
                  )}
                  {incident.software.cloud_provider && (
                    <div>
                      <p className="text-xs font-medium uppercase text-gray-500">Cloud</p>
                      <p className="mt-1 text-sm text-gray-700">{incident.software.cloud_provider}</p>
                    </div>
                  )}
                  {incident.software.description && (
                    <div className="md:col-span-2">
                      <p className="text-xs font-medium uppercase text-gray-500">Description</p>
                      <p className="mt-1 text-sm text-gray-600">{incident.software.description}</p>
                    </div>
                  )}
                  {(incident.software.sre_team?.length ?? 0) > 0 && (
                    <div>
                      <p className="text-xs font-medium uppercase text-gray-500">SRE Team</p>
                      <div className="mt-1 flex flex-wrap gap-1">
                        {incident.software.sre_team!.map((m: any, i: number) => (
                          <span key={i} className="inline-flex items-center gap-1 rounded-full bg-blue-50 px-2 py-0.5 text-xs text-blue-700">
                            <User className="h-3 w-3" />{typeof m === 'string' ? m : m.name || m.email || JSON.stringify(m)}
                          </span>
                        ))}
                      </div>
                    </div>
                  )}
                  {(incident.software.stakeholders?.length ?? 0) > 0 && (
                    <div>
                      <p className="text-xs font-medium uppercase text-gray-500">Stakeholders</p>
                      <div className="mt-1 flex flex-wrap gap-1">
                        {incident.software.stakeholders!.map((s: any, i: number) => (
                          <span key={i} className="rounded-full bg-purple-50 px-2 py-0.5 text-xs text-purple-700">
                            {typeof s === 'string' ? s : s.name || s.email || JSON.stringify(s)}
                          </span>
                        ))}
                      </div>
                    </div>
                  )}
                  {(incident.software.architects?.length ?? 0) > 0 && (
                    <div>
                      <p className="text-xs font-medium uppercase text-gray-500">Architects</p>
                      <div className="mt-1 flex flex-wrap gap-1">
                        {incident.software.architects!.map((a: any, i: number) => (
                          <span key={i} className="rounded-full bg-amber-50 px-2 py-0.5 text-xs text-amber-700">
                            {typeof a === 'string' ? a : a.name || a.email || JSON.stringify(a)}
                          </span>
                        ))}
                      </div>
                    </div>
                  )}
                  {(incident.software.tags?.length ?? 0) > 0 && (
                    <div className="md:col-span-2">
                      <p className="text-xs font-medium uppercase text-gray-500">Tags</p>
                      <div className="mt-1 flex flex-wrap gap-1">
                        {incident.software.tags!.map((t: string) => (
                          <span key={t} className="rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-600">{t}</span>
                        ))}
                      </div>
                    </div>
                  )}
                  <div className="md:col-span-2 flex flex-wrap gap-3 border-t border-gray-100 pt-3 text-xs text-gray-500">
                    {incident.software.repository_url && (
                      <a href={incident.software.repository_url} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 hover:text-blue-600">
                        <GitCommit className="h-3 w-3" /> Repository
                      </a>
                    )}
                    {incident.software.dashboard_url && (
                      <a href={incident.software.dashboard_url} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 hover:text-blue-600">
                        <Layers className="h-3 w-3" /> Dashboard
                      </a>
                    )}
                    {incident.software.runbook_url && (
                      <a href={incident.software.runbook_url} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 hover:text-blue-600">
                        <BookOpen className="h-3 w-3" /> Runbook
                      </a>
                    )}
                  </div>
                </div>
              </div>
            )}

            {/* Progress indicators with feedback */}
            <div className="rounded-lg border border-gray-200 bg-white p-5">
              <h3 className="mb-3 text-sm font-semibold text-gray-900">Analysis Progress</h3>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
                <div className="flex items-center gap-2">
                  <div className="flex-1"><ProgressIndicator label="RCI" status={incident.rci?.status} /></div>
                  {incident.rci && <FeedbackButtons incidentId={id!} targetType="rci" />}
                </div>
                <div className="flex items-center gap-2">
                  <div className="flex-1"><ProgressIndicator label="RCA" status={incident.rca?.status} /></div>
                  {incident.rca && <FeedbackButtons incidentId={id!} targetType="rca" />}
                </div>
                <div className="flex items-center gap-2">
                  <div className="flex-1"><ProgressIndicator label="Postmortem" status={incident.postmortem?.status} /></div>
                  {incident.postmortem && <FeedbackButtons incidentId={id!} targetType="postmortem" />}
                </div>
              </div>
            </div>

            {/* War Room */}
            <div className="rounded-lg border border-gray-200 bg-white p-5">
              <WarRoomPanel incidentId={id!} />
            </div>

            {/* Similar Incidents */}
            <SimilarIncidentsPanel incidentId={id!} />

            {/* Recent Changes */}
            <RecentChangesPanel incidentId={id!} incidentCreatedAt={incident.created_at} softwareId={incident.software_id} />

            {/* Correlated Alerts */}
            <CorrelatedAlertsPanel incidentId={id!} />

            {/* Orchestrator Decisions */}
            <div className="rounded-lg border border-gray-200 bg-white p-6">
              <h3 className="mb-4 text-sm font-semibold text-gray-900">Orchestrator Decisions</h3>
              <OrchestratorDecisions incidentId={id!} />
            </div>

            {/* Recent timeline */}
            <div className="rounded-lg border border-gray-200 bg-white p-6">
              <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
                <Clock className="h-4 w-4 text-gray-400" /> Recent Timeline
              </h3>
              <IncidentTimeline events={(incident.timeline ?? []).slice(-8)} />
              <div className="mt-5 border-t border-gray-200 pt-4">
                <form
                  onSubmit={(e) => {
                    e.preventDefault();
                    if (!comment.trim()) return;
                    addEventMut.mutate({ type: 'comment', data: { message: comment } });
                  }}
                  className="flex gap-2"
                >
                  <input
                    placeholder="Add a comment..."
                    value={comment}
                    onChange={(e) => setComment(e.target.value)}
                    className="flex-1 rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none"
                  />
                  <PermissionButton
                    resource="incidents" action="write"
                    type="submit"
                    disabled={addEventMut.isPending || !comment.trim()}
                    className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
                  >
                    Send
                  </PermissionButton>
                </form>
              </div>
            </div>
          </div>
        )}

        {activeTab === 'runs' && (
          <div className="space-y-6">
            <div className="rounded-lg border border-gray-200 bg-white p-6">
              <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
                <Activity className="h-4 w-4 text-gray-400" /> Investigation Timeline
              </h3>
              <RunsTimeline runs={incident.agent_runs ?? []} incident={incident} />
            </div>

            {/* A2A Tasks */}
            {(a2aTasks ?? []).length > 0 && (
              <div className="rounded-lg border border-gray-200 bg-white p-6">
                <h3 className="mb-4 text-sm font-semibold text-gray-900">A2A Tasks</h3>
                <div className="space-y-3">
                  {(a2aTasks ?? []).map((task: A2ATask) => (
                    <A2ATaskCard key={task.id} task={task} />
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        {activeTab === 'evidence' && (
          <div className="rounded-lg border border-gray-200 bg-white p-6">
            <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
              <FileSearch className="h-4 w-4 text-gray-400" />
              Evidence ({(incident.evidence ?? []).length})
            </h3>
            <EvidenceUpload incidentId={id!} />
            <div className="mt-4">
              <EvidencePanel evidence={incident.evidence ?? []} />
            </div>
          </div>
        )}

        {activeTab === 'rci-rca' && (
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
            <div className="rounded-lg border border-gray-200 bg-white p-6">
              <RCIPanel rci={incident.rci} onUpdate={(d) => rciMut.mutate(d)} />
            </div>
            <div className="rounded-lg border border-gray-200 bg-white p-6">
              <RCAPanel rca={incident.rca} onUpdate={(d) => rcaMut.mutate(d)} />
            </div>
          </div>
        )}

        {activeTab === 'postmortem' && (
          <div className="rounded-lg border border-gray-200 bg-white p-6">
            <PostmortemView postmortem={incident.postmortem} onUpdate={(d) => postmortemMut.mutate(d)} incidentId={id!} />
          </div>
        )}
      </div>
    </div>
  );
}
