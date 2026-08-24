import { useEffect, useMemo } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, Link } from 'react-router-dom';
import {
  AlertTriangle, Clock, ShieldAlert, DollarSign, ArrowRight, Bot,
} from 'lucide-react';
import {
  listIncidents,
  getAnalyticsMTTR,
  getCostByModel,
  getAnalyticsIncidentTrends,
  listA2AAgents,
  listQuarantine,
} from '@/services/api';
import type {
  Incident, Severity, A2AAgent, A2AHealthStatus,
  AnalyticsMTTR, AnalyticsCostByModel,
} from '@/types/api';
import type { QuarantinedAlert } from '@/services/api';
import { SeverityBadge } from '@/components/SeverityBadge';
import { StatusBadge } from '@/components/StatusBadge';
import { SkeletonCard, SkeletonTable } from '@/components/Skeleton';
import { useWebSocket } from '@/hooks/useWebSocket';
import { useToast } from '@/components/Toast';

const severityOrder: Severity[] = ['critical', 'high', 'medium', 'low'];
// Same hexes as SeverityBadge/AnalyticsPage -- one fixed categorical
// mapping for severity across the whole app, never reassigned per-view.
const severityColors: Record<Severity, string> = {
  critical: '#ef4444',
  high: '#f97316',
  medium: '#eab308',
  low: '#3b82f6',
};
const severityBarBg: Record<Severity, string> = {
  critical: 'bg-red-500',
  high: 'bg-orange-500',
  medium: 'bg-yellow-500',
  low: 'bg-blue-500',
};

const healthDot: Record<A2AHealthStatus, string> = {
  healthy: 'bg-green-500',
  unhealthy: 'bg-red-500',
  unknown: 'bg-gray-300',
};
const healthLabel: Record<A2AHealthStatus, string> = {
  healthy: 'Healthy',
  unhealthy: 'Unhealthy',
  unknown: 'Unknown',
};

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`;
  if (ms < 3_600_000) return `${Math.round(ms / 60_000)}m`;
  return `${(ms / 3_600_000).toFixed(1)}h`;
}

function formatMTTR(seconds: number): string {
  if (seconds < 60) return `${Math.round(seconds)}s`;
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`;
  return `${(seconds / 3600).toFixed(1)}h`;
}

function formatCost(usd: number): string {
  if (usd < 0.01) return '<$0.01';
  return `$${usd.toFixed(2)}`;
}

function formatRelativeTime(iso: string | null): string {
  if (!iso) return 'never';
  const diffMs = Date.now() - new Date(iso).getTime();
  const mins = Math.round(diffMs / 60_000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.round(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.round(hours / 24)}d ago`;
}

function incidentDuration(inc: Incident): string {
  const start = new Date(inc.created_at).getTime();
  const end = inc.resolved_at ? new Date(inc.resolved_at).getTime() : Date.now();
  return formatDuration(end - start);
}

/** 12-14 point trend line: history in the de-emphasis gray, the last
 * segment (the current period) in the accent hue, per the stat-tile
 * sparkline contract. No axes/gridlines -- it's context, not a chart. */
function Sparkline({ values, accent }: { values: number[]; accent: string }) {
  if (values.length < 2) return null;
  const w = 96;
  const h = 28;
  const max = Math.max(...values, 1);
  const min = Math.min(...values, 0);
  const range = max - min || 1;
  const points = values.map((v, i) => {
    const x = (i / (values.length - 1)) * w;
    const y = h - ((v - min) / range) * h;
    return [x, y] as const;
  });
  const toPath = (pts: readonly (readonly [number, number])[]) =>
    pts.map((p) => p.join(',')).join(' ');
  const last = points[points.length - 1];

  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="h-7 w-24 overflow-visible">
      <polyline
        points={toPath(points)}
        fill="none"
        stroke="#d1d5db"
        strokeWidth={2}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <polyline
        points={toPath(points.slice(-2))}
        fill="none"
        stroke={accent}
        strokeWidth={2}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <circle cx={last[0]} cy={last[1]} r={2.5} fill={accent} stroke="white" strokeWidth={1.5} />
    </svg>
  );
}

interface StatTileProps {
  icon: React.ComponentType<{ className?: string }>;
  iconClass: string;
  label: string;
  value: string;
  valueClass?: string;
  sparkline?: number[];
  sparklineAccent?: string;
  badge?: number;
}

function StatTile({ icon: Icon, iconClass, label, value, valueClass, sparkline, sparklineAccent, badge }: StatTileProps) {
  return (
    <div className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
      <div className="flex items-start justify-between">
        <div className={`flex h-9 w-9 items-center justify-center rounded-lg ${iconClass}`}>
          <Icon className="h-4.5 w-4.5" />
        </div>
        {typeof badge === 'number' && badge > 0 && (
          <span className="inline-flex items-center rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800">
            {badge} new
          </span>
        )}
      </div>
      <p className="mt-3 text-sm font-medium text-gray-500">{label}</p>
      <div className="mt-1 flex items-end justify-between gap-2">
        <p className={`text-3xl font-semibold ${valueClass ?? 'text-gray-900'}`}>{value}</p>
        {sparkline && sparkline.length >= 2 && (
          <Sparkline values={sparkline} accent={sparklineAccent ?? '#3b82f6'} />
        )}
      </div>
    </div>
  );
}

export function DashboardPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { lastEvent } = useWebSocket(['incident.created', 'incident.updated']);
  const { addToast } = useToast();

  // Fetch incidents (for counts + table)
  const { data: incidentsData, isLoading: incidentsLoading } = useQuery({
    queryKey: ['incidents', 'dashboard'],
    queryFn: () => listIncidents({ per_page: 100 }),
    refetchInterval: 30_000,
  });

  // MTTR analytics
  const { data: mttrData } = useQuery({
    queryKey: ['analytics', 'mttr'],
    queryFn: () => getAnalyticsMTTR(),
    refetchInterval: 60_000,
  });

  // Cost by model
  const { data: costData } = useQuery({
    queryKey: ['analytics', 'cost-by-model'],
    queryFn: () => getCostByModel(),
    refetchInterval: 60_000,
  });

  // Daily incident volume, last 14 days -- real trend data for the KPI
  // sparklines (never a fabricated delta).
  const { data: trendData } = useQuery({
    queryKey: ['analytics', 'trends', 'dashboard'],
    queryFn: () => getAnalyticsIncidentTrends({ since: new Date(Date.now() - 14 * 86_400_000).toISOString() }),
    refetchInterval: 60_000,
  });

  // A2A agents
  const { data: agentsData } = useQuery({
    queryKey: ['a2a', 'agents', 'dashboard'],
    queryFn: () => listA2AAgents({ per_page: 50 }),
    refetchInterval: 30_000,
  });

  // Quarantine alerts (unresolved)
  const { data: quarantineData } = useQuery({
    queryKey: ['quarantine', 'unresolved'],
    queryFn: () => listQuarantine({ resolved: false, per_page: 10 }),
    refetchInterval: 30_000,
  });

  useEffect(() => {
    if (!lastEvent) return;
    if (lastEvent.type === 'incident.created' || lastEvent.type === 'incident.updated') {
      queryClient.invalidateQueries({ queryKey: ['incidents', 'dashboard'] });
    }
    if (lastEvent.type === 'incident.created') {
      const eventData = lastEvent.data as { title?: string } | undefined;
      addToast({
        type: 'info',
        title: `New incident: ${eventData?.title ?? 'Unknown'}`,
        action: lastEvent.incident_id
          ? { label: 'View', href: `/incidents/${lastEvent.incident_id}` }
          : undefined,
      });
    }
  }, [lastEvent]); // eslint-disable-line react-hooks/exhaustive-deps

  const incidents = incidentsData?.data ?? [];
  const openCount = incidents.filter((i) => !['resolved', 'closed'].includes(i.status)).length;
  const bySeverity = useMemo(() => {
    const counts: Record<Severity, number> = { critical: 0, high: 0, medium: 0, low: 0 };
    incidents.forEach((i) => { counts[i.severity] = (counts[i.severity] ?? 0) + 1; });
    return counts;
  }, [incidents]);

  const maxSeverityCount = Math.max(...Object.values(bySeverity), 1);

  const avgMTTR = useMemo(() => {
    if (!mttrData || mttrData.length === 0) return null;
    const totalSeconds = mttrData.reduce((sum: number, m: AnalyticsMTTR) => sum + m.avg_mttr_seconds, 0);
    return totalSeconds / mttrData.length;
  }, [mttrData]);

  const totalCost = useMemo(() => {
    if (!costData || costData.length === 0) return null;
    return costData.reduce((sum: number, c: AnalyticsCostByModel) => sum + c.est_cost_usd, 0);
  }, [costData]);

  // One point per day, summed across severities -- daily incident volume.
  const incidentVolumeTrend = useMemo(() => {
    if (!trendData || trendData.length === 0) return [];
    const byDate = new Map<string, number>();
    trendData.forEach((d) => byDate.set(d.date, (byDate.get(d.date) ?? 0) + d.count));
    return [...byDate.entries()].sort(([a], [b]) => a.localeCompare(b)).map(([, count]) => count);
  }, [trendData]);

  const quarantineCount = quarantineData?.total ?? 0;
  const quarantineAlerts: QuarantinedAlert[] = quarantineData?.data ?? [];
  const agents: A2AAgent[] = agentsData?.data ?? [];
  const unhealthyAgents = agents.filter((a) => a.health_status !== 'healthy').length;

  if (incidentsLoading) {
    return (
      <div className="p-8">
        <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
        <p className="mt-1 text-sm text-gray-500">Overview of current incident landscape</p>
        <div className="mt-6 space-y-6">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <SkeletonCard /><SkeletonCard /><SkeletonCard /><SkeletonCard />
          </div>
          <SkeletonTable rows={5} cols={4} />
        </div>
      </div>
    );
  }

  return (
    <div className="p-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
          <p className="mt-1 text-sm text-gray-500">Overview of current incident landscape</p>
        </div>
        <div className="flex items-center gap-1.5 text-xs text-gray-400">
          <span className="h-1.5 w-1.5 rounded-full bg-green-500" />
          Auto-refreshing
        </div>
      </div>

      {/* Row 1: KPI tiles */}
      <div className="mt-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatTile
          icon={AlertTriangle}
          iconClass={openCount > 0 ? 'bg-red-50 text-red-600' : 'bg-green-50 text-green-600'}
          label="Open incidents"
          value={String(openCount)}
          valueClass={openCount > 0 ? 'text-red-600' : 'text-green-600'}
          sparkline={incidentVolumeTrend}
          sparklineAccent={openCount > 0 ? '#dc2626' : '#16a34a'}
        />
        <StatTile
          icon={Clock}
          iconClass="bg-blue-50 text-blue-600"
          label="MTTR"
          value={avgMTTR !== null ? formatMTTR(avgMTTR) : '--'}
        />
        <StatTile
          icon={ShieldAlert}
          iconClass={quarantineCount > 0 ? 'bg-amber-50 text-amber-600' : 'bg-gray-50 text-gray-500'}
          label="Quarantine"
          value={String(quarantineCount)}
          valueClass={quarantineCount > 0 ? 'text-amber-600' : 'text-gray-900'}
        />
        <StatTile
          icon={DollarSign}
          iconClass="bg-blue-50 text-blue-600"
          label="Cost (30 days)"
          value={totalCost !== null ? formatCost(totalCost) : '--'}
        />
      </div>

      {/* Row 2: Severity Chart + Agent Status */}
      <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* Incidents by Severity */}
        <div className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
          <h3 className="text-sm font-medium text-gray-900">Incidents by severity</h3>
          <div className="mt-4 space-y-3">
            {severityOrder.map((sev) => {
              const count = bySeverity[sev];
              const pct = maxSeverityCount > 0 ? (count / maxSeverityCount) * 100 : 0;
              return (
                <div key={sev} className="flex items-center gap-3">
                  <span className="w-16 shrink-0 text-sm font-medium capitalize text-gray-700">{sev}</span>
                  <div className="h-4 flex-1 rounded-full bg-gray-100">
                    <div
                      className={`h-4 rounded-full ${severityBarBg[sev]} transition-all`}
                      style={{ width: `${Math.max(pct, count > 0 ? 4 : 0)}%` }}
                    />
                  </div>
                  <span className="w-6 shrink-0 text-right text-sm font-semibold text-gray-900">{count}</span>
                </div>
              );
            })}
          </div>
        </div>

        {/* Agent Status */}
        <div className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-medium text-gray-900">Agent status</h3>
            {unhealthyAgents > 0 && (
              <span className="inline-flex items-center rounded-full bg-red-100 px-2 py-0.5 text-xs font-medium text-red-800">
                {unhealthyAgents} unhealthy
              </span>
            )}
          </div>
          {agents.length === 0 ? (
            <div className="mt-4 flex flex-col items-center justify-center py-6 text-center">
              <Bot className="h-8 w-8 text-gray-300" />
              <p className="mt-2 text-sm text-gray-500">No agents configured</p>
              <Link to="/agents" className="mt-1 text-xs font-medium text-blue-600 hover:text-blue-700">
                Set up an agent
              </Link>
            </div>
          ) : (
            <div className="mt-4 space-y-1">
              {agents.map((agent) => (
                <div
                  key={agent.id}
                  onClick={() => navigate('/agents')}
                  className="flex cursor-pointer items-center justify-between rounded px-2 py-1.5 hover:bg-gray-50"
                >
                  <div className="flex min-w-0 items-center gap-2">
                    <span
                      title={healthLabel[agent.health_status]}
                      className={`inline-block h-2.5 w-2.5 shrink-0 rounded-full ${healthDot[agent.health_status]}`}
                    />
                    <span className="truncate text-sm font-medium text-gray-900">{agent.name}</span>
                    <span className="shrink-0 text-xs capitalize text-gray-400">{agent.agent_type}</span>
                  </div>
                  <span className="shrink-0 text-xs text-gray-400">{formatRelativeTime(agent.last_health_check)}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Row 3: Recent Incidents Table */}
      <div className="mt-6">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-medium text-gray-900">Recent incidents</h3>
          <Link to="/incidents" className="flex items-center gap-1 text-xs font-medium text-blue-600 hover:text-blue-700">
            View all <ArrowRight className="h-3 w-3" />
          </Link>
        </div>
        <div className="mt-3 overflow-x-auto rounded-lg border border-gray-200 bg-white shadow-sm">
          {incidents.length === 0 ? (
            <p className="p-6 text-center text-sm text-gray-500">No incidents yet</p>
          ) : (
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">Title</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">Severity</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">Status</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">Software</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">Duration</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">Created</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {incidents.slice(0, 10).map((inc) => (
                  <tr
                    key={inc.id}
                    onClick={() => navigate(`/incidents/${inc.id}`)}
                    className="cursor-pointer border-l-[3px] hover:bg-gray-50"
                    style={{ borderLeftColor: severityColors[inc.severity] }}
                  >
                    <td className="px-4 py-3 text-sm font-medium text-gray-900">{inc.title}</td>
                    <td className="px-4 py-3"><SeverityBadge severity={inc.severity} /></td>
                    <td className="px-4 py-3"><StatusBadge status={inc.status} /></td>
                    <td className="px-4 py-3 text-sm text-gray-500">{inc.software_id || '--'}</td>
                    <td className="px-4 py-3 text-sm text-gray-500">{incidentDuration(inc)}</td>
                    <td className="px-4 py-3 text-sm text-gray-500">{new Date(inc.created_at).toLocaleDateString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>

      {/* Row 4: Quarantine Alerts */}
      {quarantineAlerts.length > 0 && (
        <div className="mt-6">
          <div className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
            <h3 className="text-sm font-medium text-gray-900">Quarantine alerts</h3>
            <div className="mt-3 space-y-2">
              {quarantineAlerts.slice(0, 5).map((alert) => (
                <div key={alert.id} className="flex items-center justify-between rounded border border-amber-100 bg-amber-50 px-3 py-2">
                  <div>
                    <p className="text-sm font-medium text-gray-900">{alert.reason}</p>
                    <p className="text-xs text-gray-500">Source: {alert.source} | {new Date(alert.created_at).toLocaleDateString()}</p>
                  </div>
                  <button
                    onClick={() => navigate('/quarantine')}
                    className="rounded bg-amber-100 px-2 py-1 text-xs font-medium text-amber-800 hover:bg-amber-200"
                  >
                    Link
                  </button>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
