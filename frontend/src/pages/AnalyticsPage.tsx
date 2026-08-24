import { useQuery } from '@tanstack/react-query';
import { BarChart3, TrendingUp, Bot, DollarSign } from 'lucide-react';
import { getAnalyticsMTTR, getAnalyticsIncidentTrends, getAnalyticsAgentEffectiveness, getCostByModel, getCostByIncident } from '@/services/api';
import type { AnalyticsMTTR, AnalyticsIncidentTrend, AnalyticsAgentEffectiveness, AnalyticsCostByModel, AnalyticsCostByIncident } from '@/types/api';
import { Link } from 'react-router-dom';

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${Math.round(seconds)}s`;
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`;
  const h = Math.floor(seconds / 3600);
  const m = Math.round((seconds % 3600) / 60);
  return `${h}h ${m}m`;
}

function MTTRChart({ data }: { data: AnalyticsMTTR[] }) {
  if (!data.length) return <p className="text-sm text-gray-400">No data available</p>;

  const maxVal = Math.max(...data.map((d) => d.avg_mttr_seconds));
  const chartW = 600;
  const chartH = 280;
  const barW = Math.min(60, (chartW - 40) / data.length - 10);
  const padLeft = 60;
  const padBottom = 80;
  const plotH = chartH - padBottom - 20;

  return (
    <svg viewBox={`0 0 ${chartW} ${chartH}`} className="w-full max-w-2xl">
      {/* Grid lines */}
      {[0, 0.25, 0.5, 0.75, 1].map((f) => {
        const y = 20 + plotH * (1 - f);
        return (
          <g key={f}>
            <line x1={padLeft} y1={y} x2={chartW - 10} y2={y} stroke="#e5e7eb" strokeWidth={1} />
            <text x={padLeft - 8} y={y + 4} textAnchor="end" className="fill-gray-400" fontSize={10}>
              {formatDuration(maxVal * f)}
            </text>
          </g>
        );
      })}
      {/* Bars */}
      {data.map((d, i) => {
        const x = padLeft + 20 + i * ((chartW - padLeft - 30) / data.length);
        const h = maxVal > 0 ? (d.avg_mttr_seconds / maxVal) * plotH : 0;
        const y = 20 + plotH - h;
        return (
          <g key={d.software_id}>
            <rect x={x} y={y} width={barW} height={h} rx={4} className="fill-blue-500" opacity={0.85} />
            <text x={x + barW / 2} y={y - 6} textAnchor="middle" fontSize={10} className="fill-gray-700 font-medium">
              {formatDuration(d.avg_mttr_seconds)}
            </text>
            <text
              x={x + barW / 2}
              y={chartH - padBottom + 16}
              textAnchor="middle"
              fontSize={10}
              className="fill-gray-600"
              transform={`rotate(-30, ${x + barW / 2}, ${chartH - padBottom + 16})`}
            >
              {d.software_name.length > 12 ? d.software_name.slice(0, 12) + '...' : d.software_name}
            </text>
            <text x={x + barW / 2} y={chartH - padBottom + 32} textAnchor="middle" fontSize={9} className="fill-gray-400">
              ({d.incident_count})
            </text>
          </g>
        );
      })}
    </svg>
  );
}

const severityColors: Record<string, string> = {
  critical: '#ef4444',
  high: '#f97316',
  medium: '#eab308',
  low: '#3b82f6',
};

function TrendChart({ data }: { data: AnalyticsIncidentTrend[] }) {
  if (!data.length) return <p className="text-sm text-gray-400">No data available</p>;

  const dates = [...new Set(data.map((d) => d.date))].sort();
  const severities = [...new Set(data.map((d) => d.severity))];
  const maxCount = Math.max(...dates.map((date) => data.filter((d) => d.date === date).reduce((s, d) => s + d.count, 0)), 1);

  const chartW = 700;
  const chartH = 280;
  const padLeft = 50;
  const padBottom = 50;
  const padTop = 20;
  const plotW = chartW - padLeft - 20;
  const plotH = chartH - padBottom - padTop;

  const getX = (i: number) => padLeft + (i / Math.max(dates.length - 1, 1)) * plotW;
  const getY = (v: number) => padTop + plotH * (1 - v / maxCount);

  return (
    <svg viewBox={`0 0 ${chartW} ${chartH}`} className="w-full">
      {/* Grid */}
      {[0, 0.25, 0.5, 0.75, 1].map((f) => {
        const y = getY(maxCount * f);
        return (
          <g key={f}>
            <line x1={padLeft} y1={y} x2={chartW - 20} y2={y} stroke="#e5e7eb" strokeWidth={1} />
            <text x={padLeft - 8} y={y + 4} textAnchor="end" fontSize={10} className="fill-gray-400">
              {Math.round(maxCount * f)}
            </text>
          </g>
        );
      })}
      {/* X labels */}
      {dates.filter((_, i) => i % Math.max(1, Math.floor(dates.length / 7)) === 0).map((date) => {
        const i = dates.indexOf(date);
        return (
          <text key={date} x={getX(i)} y={chartH - padBottom + 18} textAnchor="middle" fontSize={10} className="fill-gray-400">
            {date.slice(5)}
          </text>
        );
      })}
      {/* Lines per severity */}
      {severities.map((sev) => {
        const points = dates.map((date, i) => {
          const entry = data.find((d) => d.date === date && d.severity === sev);
          return `${getX(i)},${getY(entry?.count ?? 0)}`;
        });
        return (
          <polyline
            key={sev}
            points={points.join(' ')}
            fill="none"
            stroke={severityColors[sev] ?? '#6b7280'}
            strokeWidth={2}
            strokeLinejoin="round"
          />
        );
      })}
      {/* Legend */}
      {severities.map((sev, i) => (
        <g key={sev} transform={`translate(${padLeft + i * 90}, ${chartH - 10})`}>
          <rect width={12} height={12} rx={2} fill={severityColors[sev] ?? '#6b7280'} />
          <text x={16} y={10} fontSize={10} className="fill-gray-600 capitalize">{sev}</text>
        </g>
      ))}
    </svg>
  );
}

function AgentTable({ data }: { data: AnalyticsAgentEffectiveness[] }) {
  if (!data.length) return <p className="text-sm text-gray-400">No data available</p>;

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-gray-200 text-left text-xs font-medium uppercase text-gray-500">
            <th className="px-4 py-3">Agent Name</th>
            <th className="px-4 py-3">Total Tasks</th>
            <th className="px-4 py-3">Success Rate</th>
            <th className="px-4 py-3">Avg Duration</th>
          </tr>
        </thead>
        <tbody>
          {data.map((a) => (
            <tr key={a.agent_name} className="border-b border-gray-100 hover:bg-gray-50">
              <td className="px-4 py-3 font-medium text-gray-900">{a.agent_name}</td>
              <td className="px-4 py-3 text-gray-600">{a.total_tasks.toLocaleString()}</td>
              <td className="px-4 py-3">
                <div className="flex items-center gap-2">
                  <div className="h-2 w-24 rounded-full bg-gray-200">
                    <div
                      className={`h-2 rounded-full ${a.success_rate >= 0.9 ? 'bg-green-500' : a.success_rate >= 0.7 ? 'bg-amber-500' : 'bg-red-500'}`}
                      style={{ width: `${Math.round(a.success_rate * 100)}%` }}
                    />
                  </div>
                  <span className="text-xs text-gray-600">{Math.round(a.success_rate * 100)}%</span>
                </div>
              </td>
              <td className="px-4 py-3 text-gray-600">
                {a.avg_duration_ms < 1000 ? `${Math.round(a.avg_duration_ms)}ms` : `${(a.avg_duration_ms / 1000).toFixed(1)}s`}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function getModelBarColor(model: string): string {
  const m = model.toLowerCase();
  if (m.includes('sonnet') || m.includes('claude')) return 'bg-blue-500';
  if (m.includes('llama')) return 'bg-green-500';
  return 'bg-gray-400';
}

function formatUsd(amount: number): string {
  if (amount < 0.01) return `$${amount.toFixed(4)}`;
  return `$${amount.toFixed(2)}`;
}

function formatTokens(n: number): string {
  return n.toLocaleString();
}

function formatDurationMs(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)}ms`;
  const totalSec = Math.floor(ms / 1000);
  if (totalSec < 60) return `${totalSec}s`;
  const m = Math.floor(totalSec / 60);
  const s = totalSec % 60;
  return `${m}m ${s}s`;
}

function CostByModelCard({ data }: { data: AnalyticsCostByModel[] }) {
  if (!data.length) return <p className="text-sm text-gray-400">No cost data available</p>;

  const totalCost = data.reduce((s, d) => s + d.est_cost_usd, 0);
  const totalTokens = data.reduce((s, d) => s + d.total_tokens, 0);
  const totalRuns = data.reduce((s, d) => s + d.total_runs, 0);
  const maxCost = Math.max(...data.map((d) => d.est_cost_usd));

  return (
    <div>
      <div className="mb-4 text-center">
        <p className="text-3xl font-bold text-gray-900">{formatUsd(totalCost)}</p>
        <p className="text-xs text-gray-500">Total Estimated Cost</p>
      </div>
      <div className="space-y-3">
        {data.map((d) => (
          <div key={d.model} className="flex items-center gap-3">
            <span className="w-28 truncate text-xs font-medium text-gray-700" title={d.model}>{d.model}</span>
            <div className="flex-1">
              <div className="h-5 w-full rounded bg-gray-100">
                <div
                  className={`h-5 rounded ${getModelBarColor(d.model)}`}
                  style={{ width: `${maxCost > 0 ? (d.est_cost_usd / maxCost) * 100 : 0}%`, minWidth: '2px' }}
                />
              </div>
            </div>
            <span className="w-16 text-right text-xs font-semibold text-gray-800">{formatUsd(d.est_cost_usd)}</span>
          </div>
        ))}
      </div>
      <div className="mt-4 flex gap-6 border-t border-gray-100 pt-3">
        <div>
          <p className="text-xs text-gray-500">Total Tokens</p>
          <p className="text-sm font-medium text-gray-700">{formatTokens(totalTokens)}</p>
        </div>
        <div>
          <p className="text-xs text-gray-500">Total Runs</p>
          <p className="text-sm font-medium text-gray-700">{totalRuns.toLocaleString()}</p>
        </div>
      </div>
    </div>
  );
}

function CostByIncidentTable({ data }: { data: AnalyticsCostByIncident[] }) {
  if (!data.length) return <p className="text-sm text-gray-400">No incident cost data available</p>;

  const sorted = [...data].sort((a, b) => b.created_at.localeCompare(a.created_at));

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-gray-200 text-left text-xs font-medium uppercase text-gray-500">
            <th className="px-3 py-3">Incident</th>
            <th className="px-3 py-3">Runs</th>
            <th className="px-3 py-3">Tokens</th>
            <th className="px-3 py-3">Est. Cost</th>
            <th className="px-3 py-3">Duration</th>
            <th className="px-3 py-3">Date</th>
          </tr>
        </thead>
        <tbody>
          {sorted.map((d) => (
            <tr key={d.incident_id} className="border-b border-gray-100 hover:bg-gray-50">
              <td className="max-w-[180px] truncate px-3 py-3 font-medium text-blue-600">
                <Link to={`/incidents/${d.incident_id}`} title={d.incident_title}>
                  {d.incident_title.length > 40 ? d.incident_title.slice(0, 40) + '...' : d.incident_title}
                </Link>
              </td>
              <td className="px-3 py-3 text-gray-600">{d.total_runs}</td>
              <td className="px-3 py-3 text-gray-600">{formatTokens(d.total_tokens)}</td>
              <td className="px-3 py-3 font-medium text-gray-900">{formatUsd(d.est_cost_usd)}</td>
              <td className="px-3 py-3 text-gray-600">{formatDurationMs(d.total_duration_ms)}</td>
              <td className="px-3 py-3 text-gray-500">{new Date(d.created_at).toLocaleDateString()}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function CostMetricCards({ modelData, incidentData }: { modelData: AnalyticsCostByModel[]; incidentData: AnalyticsCostByIncident[] }) {
  const totalCost = modelData.reduce((s, d) => s + d.est_cost_usd, 0);
  const totalTokens = modelData.reduce((s, d) => s + d.total_tokens, 0);
  const totalRuns = modelData.reduce((s, d) => s + d.total_runs, 0);
  const avgCostPerIncident = incidentData.length > 0 ? totalCost / incidentData.length : 0;

  const metrics = [
    { label: 'Total Cost', value: formatUsd(totalCost), color: 'text-blue-600' },
    { label: 'Total Tokens', value: formatTokens(totalTokens), color: 'text-gray-900' },
    { label: 'Avg Cost / Incident', value: formatUsd(avgCostPerIncident), color: 'text-amber-600' },
    { label: 'Total Runs', value: totalRuns.toLocaleString(), color: 'text-purple-600' },
  ];

  return (
    <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
      {metrics.map((m) => (
        <div key={m.label} className="rounded-lg border border-gray-200 bg-white p-4 text-center">
          <p className={`text-xl font-bold ${m.color}`}>{m.value}</p>
          <p className="text-xs text-gray-500">{m.label}</p>
        </div>
      ))}
    </div>
  );
}

export function AnalyticsPage() {
  const { data: mttrData, isLoading: mttrLoading } = useQuery({
    queryKey: ['analytics-mttr'],
    queryFn: () => getAnalyticsMTTR(),
  });

  const { data: trendData, isLoading: trendLoading } = useQuery({
    queryKey: ['analytics-trends'],
    queryFn: () => getAnalyticsIncidentTrends(),
  });

  const { data: agentData, isLoading: agentLoading } = useQuery({
    queryKey: ['analytics-agents'],
    queryFn: () => getAnalyticsAgentEffectiveness(),
  });

  const { data: costByModelData, isLoading: costModelLoading } = useQuery({
    queryKey: ['analytics-cost-model'],
    queryFn: () => getCostByModel(),
  });

  const { data: costByIncidentData, isLoading: costIncidentLoading } = useQuery({
    queryKey: ['analytics-cost-incident'],
    queryFn: () => getCostByIncident(),
  });

  const costLoading = costModelLoading || costIncidentLoading;

  return (
    <div className="p-8">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Analytics</h1>
        <p className="mt-1 text-sm text-gray-500">Incident metrics, trends, and agent performance</p>
      </div>

      <div className="space-y-6">
        {/* MTTR Overview */}
        <div className="rounded-lg border border-gray-200 bg-white p-6">
          <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
            <BarChart3 className="h-4 w-4 text-blue-500" /> MTTR by Software
          </h3>
          {mttrLoading ? (
            <div className="flex h-40 items-center justify-center">
              <div className="h-6 w-6 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
            </div>
          ) : (
            <MTTRChart data={mttrData ?? []} />
          )}
        </div>

        {/* Incident Trends */}
        <div className="rounded-lg border border-gray-200 bg-white p-6">
          <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
            <TrendingUp className="h-4 w-4 text-green-500" /> Incident Trends (30 days)
          </h3>
          {trendLoading ? (
            <div className="flex h-40 items-center justify-center">
              <div className="h-6 w-6 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
            </div>
          ) : (
            <TrendChart data={trendData ?? []} />
          )}
        </div>

        {/* Agent Effectiveness */}
        <div className="rounded-lg border border-gray-200 bg-white p-6">
          <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
            <Bot className="h-4 w-4 text-purple-500" /> Agent Effectiveness
          </h3>
          {agentLoading ? (
            <div className="flex h-40 items-center justify-center">
              <div className="h-6 w-6 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
            </div>
          ) : (
            <AgentTable data={agentData ?? []} />
          )}
        </div>

        {/* Cost Analysis */}
        <div>
          <h2 className="mb-4 flex items-center gap-2 text-lg font-semibold text-gray-900">
            <DollarSign className="h-5 w-5 text-green-500" /> Cost Analysis
          </h2>

          {costLoading ? (
            <div className="flex h-40 items-center justify-center">
              <div className="h-6 w-6 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
            </div>
          ) : (
            <>
              <CostMetricCards modelData={costByModelData ?? []} incidentData={costByIncidentData ?? []} />

              <div className="mt-4 grid gap-4 lg:grid-cols-5">
                <div className="rounded-lg border border-gray-200 bg-white p-6 lg:col-span-2">
                  <h3 className="mb-4 text-sm font-semibold text-gray-900">Cost by Model</h3>
                  <CostByModelCard data={costByModelData ?? []} />
                </div>
                <div className="rounded-lg border border-gray-200 bg-white p-6 lg:col-span-3">
                  <h3 className="mb-4 text-sm font-semibold text-gray-900">Cost by Incident</h3>
                  <CostByIncidentTable data={costByIncidentData ?? []} />
                </div>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
