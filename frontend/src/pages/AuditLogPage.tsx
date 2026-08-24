import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { listAuditLog, listUsers } from '@/services/api';
import { ChevronDown, ChevronRight } from 'lucide-react';
import type { AuditLogEntry } from '@/types/api';

const ACTION_COLORS: Record<string, string> = {
  create: 'bg-green-100 text-green-700',
  update: 'bg-blue-100 text-blue-700',
  delete: 'bg-red-100 text-red-700',
  login: 'bg-purple-100 text-purple-700',
  logout: 'bg-gray-100 text-gray-600',
};

function getActionColor(action: string): string {
  for (const [key, cls] of Object.entries(ACTION_COLORS)) {
    if (action.toLowerCase().includes(key)) return cls;
  }
  return 'bg-gray-100 text-gray-600';
}

function formatRelativeTime(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function ExpandableRow({ entry, userName }: { entry: AuditLogEntry; userName?: string }) {
  const [expanded, setExpanded] = useState(false);

  return (
    <>
      <tr className="cursor-pointer hover:bg-gray-50" onClick={() => setExpanded(!expanded)}>
        <td className="whitespace-nowrap px-4 py-3 text-sm">
          <div className="flex items-center gap-1">
            {expanded ? <ChevronDown className="h-3 w-3 text-gray-400" /> : <ChevronRight className="h-3 w-3 text-gray-400" />}
            <span className="font-mono text-xs text-gray-500" title={new Date(entry.created_at).toISOString()}>
              {formatRelativeTime(entry.created_at)}
            </span>
          </div>
        </td>
        <td className="whitespace-nowrap px-4 py-3 text-sm text-gray-700">
          {userName ?? entry.user_id?.slice(0, 8) ?? '-'}
        </td>
        <td className="whitespace-nowrap px-4 py-3">
          <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${getActionColor(entry.action)}`}>{entry.action}</span>
        </td>
        <td className="whitespace-nowrap px-4 py-3 text-sm text-gray-700">
          {entry.resource_type}{entry.resource_id ? ` / ${entry.resource_id.slice(0, 8)}...` : ''}
        </td>
        <td className="whitespace-nowrap px-4 py-3 font-mono text-xs text-gray-500">{entry.ip_address ?? '-'}</td>
        <td className="whitespace-nowrap px-4 py-3 font-mono text-xs text-gray-400">{entry.request_id?.slice(0, 8) ?? '-'}</td>
      </tr>
      {expanded && (
        <tr>
          <td colSpan={6} className="bg-gray-50 px-8 py-4">
            <pre className="max-h-48 overflow-auto rounded bg-gray-900 p-3 font-mono text-xs text-green-400">
              {JSON.stringify(entry.details, null, 2)}
            </pre>
          </td>
        </tr>
      )}
    </>
  );
}

export function AuditLogPage() {
  const [userId, setUserId] = useState('');
  const [action, setAction] = useState('');
  const [resourceType, setResourceType] = useState('');
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [page, setPage] = useState(1);

  const { data: usersData } = useQuery({ queryKey: ['users-list'], queryFn: () => listUsers({ per_page: 200 }) });
  const userMap = new Map((usersData?.data ?? []).map((u) => [u.id, u.name]));

  const params: Record<string, string | number> = { page, per_page: 50 };
  if (userId) params.user_id = userId;
  if (action) params.action = action;
  if (resourceType) params.resource_type = resourceType;
  if (from) params.from = from;
  if (to) params.to = to;

  const { data, isLoading } = useQuery({
    queryKey: ['audit-log', params],
    queryFn: () => listAuditLog(params as Parameters<typeof listAuditLog>[0]),
  });

  const entries = data?.data ?? [];
  const total = data?.total ?? 0;

  return (
    <div className="p-8">
      <h1 className="text-2xl font-bold text-gray-900">Audit Log</h1>
      <p className="mt-1 text-sm text-gray-500">Track all actions performed in the system</p>

      {/* Filters */}
      <div className="mt-6 flex flex-wrap items-end gap-3">
        <div>
          <label className="block text-xs font-medium text-gray-500">User</label>
          <select value={userId} onChange={(e) => { setUserId(e.target.value); setPage(1); }} className="mt-1 rounded-md border border-gray-300 px-3 py-1.5 text-sm">
            <option value="">All users</option>
            {(usersData?.data ?? []).map((u) => <option key={u.id} value={u.id}>{u.name}</option>)}
          </select>
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-500">Action</label>
          <select value={action} onChange={(e) => { setAction(e.target.value); setPage(1); }} className="mt-1 rounded-md border border-gray-300 px-3 py-1.5 text-sm">
            <option value="">All actions</option>
            {['create', 'update', 'delete', 'login', 'logout'].map((a) => <option key={a} value={a}>{a}</option>)}
          </select>
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-500">Resource Type</label>
          <select value={resourceType} onChange={(e) => { setResourceType(e.target.value); setPage(1); }} className="mt-1 rounded-md border border-gray-300 px-3 py-1.5 text-sm">
            <option value="">All types</option>
            {['user', 'role', 'software', 'incident', 'agent', 'webhook', 'api_key', 'sso_provider'].map((t) => <option key={t} value={t}>{t}</option>)}
          </select>
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-500">From</label>
          <input type="date" value={from} onChange={(e) => { setFrom(e.target.value); setPage(1); }} className="mt-1 rounded-md border border-gray-300 px-3 py-1.5 text-sm" />
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-500">To</label>
          <input type="date" value={to} onChange={(e) => { setTo(e.target.value); setPage(1); }} className="mt-1 rounded-md border border-gray-300 px-3 py-1.5 text-sm" />
        </div>
      </div>

      {/* Table */}
      {isLoading ? (
        <div className="mt-6 text-sm text-gray-500">Loading audit log...</div>
      ) : (
        <>
          <div className="mt-4 overflow-x-auto rounded-lg border border-gray-200 bg-white shadow-sm">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Timestamp</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">User</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Action</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Resource</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">IP</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Request ID</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {entries.map((entry) => (
                  <ExpandableRow key={entry.id} entry={entry} userName={entry.user_id ? userMap.get(entry.user_id) : undefined} />
                ))}
                {entries.length === 0 && (
                  <tr><td colSpan={6} className="px-4 py-8 text-center text-sm text-gray-400">No audit log entries found</td></tr>
                )}
              </tbody>
            </table>
          </div>
          {total > 50 && (
            <div className="mt-3 flex items-center justify-between text-sm text-gray-500">
              <span>Showing {(page - 1) * 50 + 1}-{Math.min(page * 50, total)} of {total}</span>
              <div className="flex gap-2">
                <button disabled={page <= 1} onClick={() => setPage(page - 1)} className="rounded border px-3 py-1 disabled:opacity-50">Prev</button>
                <button disabled={page * 50 >= total} onClick={() => setPage(page + 1)} className="rounded border px-3 py-1 disabled:opacity-50">Next</button>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
