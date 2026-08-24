import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  listNotificationChannels,
  createNotificationChannel,
  deleteNotificationChannel,
  listNotificationLogs,
} from '@/services/api';
import type { NotificationChannel } from '@/types/api';

const CHANNEL_LABELS: Record<string, string> = {
  slack: 'Slack',
  teams: 'Microsoft Teams',
  pagerduty: 'PagerDuty',
  email: 'Email',
  webhook: 'Webhook',
};

const configFields: Record<string, { label: string; key: string; placeholder: string }[]> = {
  slack: [{ label: 'Webhook URL', key: 'webhook_url', placeholder: 'https://hooks.slack.com/...' }],
  teams: [{ label: 'Webhook URL', key: 'webhook_url', placeholder: 'https://outlook.office.com/webhook/...' }],
  pagerduty: [{ label: 'Routing Key', key: 'routing_key', placeholder: 'your-pagerduty-routing-key' }],
  email: [{ label: 'Email Address', key: 'email', placeholder: 'ops@company.com' }],
  webhook: [{ label: 'URL', key: 'url', placeholder: 'https://your-endpoint.com/hook' }],
};

export function NotificationChannelsPage() {
  const qc = useQueryClient();
  const [showCreate, setShowCreate] = useState(false);
  const [form, setForm] = useState<{
    name: string;
    channel_type: string;
    config: Record<string, string>;
  }>({ name: '', channel_type: 'slack', config: {} });

  const { data, isLoading } = useQuery({
    queryKey: ['notification-channels'],
    queryFn: () => listNotificationChannels(),
  });

  const { data: logsData } = useQuery({
    queryKey: ['notification-logs'],
    queryFn: () => listNotificationLogs(),
  });

  const createMut = useMutation({
    mutationFn: (d: Partial<NotificationChannel>) => createNotificationChannel(d),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['notification-channels'] });
      setShowCreate(false);
      setForm({ name: '', channel_type: 'slack', config: {} });
    },
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => deleteNotificationChannel(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['notification-channels'] }),
  });

  const channels: NotificationChannel[] = data?.data ?? [];
  const logs = logsData?.data ?? [];

  return (
    <div className="p-8 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Notification Channels</h1>
          <p className="text-gray-500 text-sm mt-1">Alert destinations for incident events</p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded-lg font-medium"
        >
          + Add Channel
        </button>
      </div>

      {showCreate && (
        <div className="bg-white border border-gray-200 rounded-lg p-5 shadow-sm space-y-4">
          <h3 className="text-gray-900 font-medium">New Notification Channel</h3>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-gray-500 text-xs mb-1 block font-medium">Name</label>
              <input
                value={form.name}
                onChange={e => setForm(p => ({ ...p, name: e.target.value }))}
                className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder="e.g., #alerts-slack"
              />
            </div>
            <div>
              <label className="text-gray-500 text-xs mb-1 block font-medium">Type</label>
              <select
                value={form.channel_type}
                onChange={e => setForm(p => ({ ...p, channel_type: e.target.value, config: {} }))}
                className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                {Object.keys(CHANNEL_LABELS).map(t => (
                  <option key={t} value={t}>{CHANNEL_LABELS[t]}</option>
                ))}
              </select>
            </div>
          </div>
          {configFields[form.channel_type]?.map(f => (
            <div key={f.key}>
              <label className="text-gray-500 text-xs mb-1 block font-medium">{f.label}</label>
              <input
                value={form.config[f.key] ?? ''}
                onChange={e => setForm(p => ({ ...p, config: { ...p.config, [f.key]: e.target.value } }))}
                className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder={f.placeholder}
              />
            </div>
          ))}
          <div className="flex gap-2">
            <button
              onClick={() => createMut.mutate({ ...form, channel_type: form.channel_type as NotificationChannel['channel_type'] })}
              disabled={!form.name || createMut.isPending}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white text-sm rounded font-medium"
            >
              {createMut.isPending ? 'Saving...' : 'Save'}
            </button>
            <button
              onClick={() => setShowCreate(false)}
              className="px-4 py-2 bg-gray-100 hover:bg-gray-200 text-gray-700 text-sm rounded"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {isLoading ? (
        <div className="flex items-center justify-center h-40">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
        </div>
      ) : channels.length === 0 ? (
        <div className="text-center py-16 text-gray-400">
          <p className="text-lg font-medium text-gray-600">No notification channels configured</p>
          <p className="text-sm mt-1">Add Slack, Teams, PagerDuty, or webhook destinations.</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {channels.map(ch => (
            <div key={ch.id} className="bg-white border border-gray-200 rounded-lg p-4 flex items-center gap-4 hover:shadow-sm transition-shadow">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <p className="text-gray-900 font-medium">{ch.name}</p>
                  <span className={`w-2 h-2 rounded-full ${ch.enabled ? 'bg-green-500' : 'bg-gray-400'}`} />
                </div>
                <p className="text-gray-500 text-sm">{CHANNEL_LABELS[ch.channel_type] ?? ch.channel_type}</p>
              </div>
              <button
                onClick={() => deleteMut.mutate(ch.id)}
                disabled={deleteMut.isPending}
                className="text-gray-400 hover:text-red-500 transition-colors text-sm px-2"
                title="Remove channel"
              >
                ✕
              </button>
            </div>
          ))}
        </div>
      )}

      {logs.length > 0 && (
        <div>
          <h2 className="text-lg font-semibold text-gray-900 mb-3">Recent Notification Logs</h2>
          <div className="bg-white border border-gray-200 rounded-lg overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-gray-200 text-left text-xs font-medium uppercase text-gray-500">
                    <th className="px-4 py-3">Event</th>
                    <th className="px-4 py-3">Recipient</th>
                    <th className="px-4 py-3">Status</th>
                    <th className="px-4 py-3">Sent At</th>
                  </tr>
                </thead>
                <tbody>
                  {logs.slice(0, 20).map(log => (
                    <tr key={log.id} className="border-b border-gray-100 hover:bg-gray-50">
                      <td className="px-4 py-3 text-gray-700">{log.event_type}</td>
                      <td className="px-4 py-3 text-gray-600">{log.recipient ?? '—'}</td>
                      <td className="px-4 py-3">
                        <span className={`px-2 py-0.5 rounded text-xs font-medium ${
                          log.status === 'sent' ? 'bg-green-100 text-green-700' :
                          log.status === 'failed' ? 'bg-red-100 text-red-700' :
                          'bg-gray-100 text-gray-600'
                        }`}>
                          {log.status}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-gray-500 text-xs">
                        {log.sent_at ? new Date(log.sent_at).toLocaleString() : '—'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
