import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Bell, Plus, X, Hash, MessageSquare, AlertCircle, Mail, Globe, Link as LinkIcon } from 'lucide-react';
import { useToast } from '@/components/Toast';
import {
  listNotificationChannels, createNotificationChannel, updateNotificationChannel, deleteNotificationChannel,
  listEscalationPolicies, createEscalationPolicy, deleteEscalationPolicy,
  listNotificationLogs,
  listSoftware,
} from '@/services/api';
import type { NotificationChannel, NotificationChannelType, EscalationPolicy, EscalationStep } from '@/types/api';

type TabKey = 'channels' | 'policies' | 'logs';

const channelTypeConfig: Record<NotificationChannelType, { icon: typeof Hash; color: string; bg: string; label: string }> = {
  slack: { icon: Hash, color: 'text-[#4A154B]', bg: 'bg-[#4A154B]/10', label: 'Slack' },
  teams: { icon: MessageSquare, color: 'text-[#6264A7]', bg: 'bg-[#6264A7]/10', label: 'Teams' },
  pagerduty: { icon: AlertCircle, color: 'text-[#06AC38]', bg: 'bg-[#06AC38]/10', label: 'PagerDuty' },
  email: { icon: Mail, color: 'text-gray-600', bg: 'bg-gray-100', label: 'Email' },
  webhook: { icon: Globe, color: 'text-blue-600', bg: 'bg-blue-50', label: 'Webhook' },
};

const channelConfigFields: Record<NotificationChannelType, { key: string; label: string; type?: string }[]> = {
  slack: [{ key: 'webhook_url', label: 'Webhook URL' }, { key: 'channel', label: 'Channel' }],
  teams: [{ key: 'webhook_url', label: 'Webhook URL' }],
  pagerduty: [{ key: 'routing_key', label: 'Routing Key' }, { key: 'severity', label: 'Default Severity' }],
  email: [{ key: 'smtp_host', label: 'SMTP Host' }, { key: 'smtp_port', label: 'SMTP Port' }, { key: 'from_address', label: 'From Address' }],
  webhook: [{ key: 'url', label: 'URL' }, { key: 'method', label: 'Method' }],
};

function ChannelsTab() {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState('');
  const [newType, setNewType] = useState<NotificationChannelType>('slack');
  const [newConfig, setNewConfig] = useState<Record<string, string>>({});

  const { data: channelsData, isLoading } = useQuery({
    queryKey: ['notification-channels'],
    queryFn: () => listNotificationChannels(),
  });

  const createMut = useMutation({
    mutationFn: (data: Partial<NotificationChannel>) => createNotificationChannel(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notification-channels'] });
      setShowCreate(false);
      setNewName('');
      setNewConfig({});
      addToast({ type: 'success', title: 'Notification channel created successfully' });
    },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to create notification channel', message: err?.response?.data?.error || err.message }); },
  });

  const toggleMut = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) => updateNotificationChannel(id, { enabled }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['notification-channels'] }); addToast({ type: 'success', title: 'Channel status updated' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to update channel', message: err?.response?.data?.error || err.message }); },
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => deleteNotificationChannel(id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['notification-channels'] }); addToast({ type: 'success', title: 'Notification channel deleted' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to delete notification channel', message: err?.response?.data?.error || err.message }); },
  });

  const channels = channelsData?.data ?? [];

  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <p className="text-sm text-gray-500">Manage notification delivery channels</p>
        <button onClick={() => setShowCreate(true)}
          className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white hover:bg-blue-700">
          <Plus className="h-4 w-4" /> Add Channel
        </button>
      </div>

      {isLoading ? (
        <div className="flex h-32 items-center justify-center">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
        </div>
      ) : channels.length === 0 ? (
        <div className="rounded-lg border border-gray-200 bg-white p-8 text-center">
          <Bell className="mx-auto h-10 w-10 text-gray-300" />
          <p className="mt-2 text-sm text-gray-500">No channels configured</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
          {channels.map((ch) => {
            const cfg = channelTypeConfig[ch.channel_type] ?? channelTypeConfig.webhook;
            const Icon = cfg.icon;
            return (
              <div key={ch.id} className="rounded-lg border border-gray-200 bg-white p-5">
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-3">
                    <div className={`flex h-10 w-10 items-center justify-center rounded-lg ${cfg.bg}`}>
                      <Icon className={`h-5 w-5 ${cfg.color}`} />
                    </div>
                    <div>
                      <h4 className="text-sm font-semibold text-gray-900">{ch.name}</h4>
                      <span className="text-xs text-gray-500">{cfg.label}</span>
                    </div>
                  </div>
                  <button
                    onClick={() => toggleMut.mutate({ id: ch.id, enabled: !ch.enabled })}
                    className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                      ch.enabled ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'
                    }`}
                  >
                    {ch.enabled ? 'Active' : 'Inactive'}
                  </button>
                </div>
                <div className="mt-3 flex justify-end">
                  <button onClick={() => { if (confirm('Delete channel?')) deleteMut.mutate(ch.id); }}
                    className="text-xs text-red-500 hover:text-red-700">Delete</button>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {showCreate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-full max-w-lg rounded-lg bg-white p-6 shadow-xl">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-lg font-semibold text-gray-900">Add Channel</h2>
              <button onClick={() => setShowCreate(false)} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
            </div>
            <form onSubmit={(e) => {
              e.preventDefault();
              createMut.mutate({ name: newName, channel_type: newType, config: newConfig, enabled: true });
            }} className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-gray-700">Name</label>
                <input value={newName} onChange={(e) => setNewName(e.target.value)} required
                  className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-700">Type</label>
                <select value={newType} onChange={(e) => { setNewType(e.target.value as NotificationChannelType); setNewConfig({}); }}
                  className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none">
                  {Object.entries(channelTypeConfig).map(([k, v]) => <option key={k} value={k}>{v.label}</option>)}
                </select>
              </div>
              {channelConfigFields[newType]?.map((f) => (
                <div key={f.key}>
                  <label className="block text-xs font-medium text-gray-700">{f.label}</label>
                  <input value={newConfig[f.key] ?? ''} onChange={(e) => setNewConfig({ ...newConfig, [f.key]: e.target.value })}
                    className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
                </div>
              ))}
              <div className="flex justify-end gap-2">
                <button type="button" onClick={() => setShowCreate(false)}
                  className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
                <button type="submit" disabled={createMut.isPending || !newName.trim()}
                  className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">Create</button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

function PoliciesTab() {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState('');
  const [newSoftwareId, setNewSoftwareId] = useState('');
  const [newSeverities, setNewSeverities] = useState<string[]>([]);
  const [newSteps, setNewSteps] = useState<EscalationStep[]>([{ delay_seconds: 0, channel_id: '', recipients: [] }]);

  const { data: policiesData, isLoading } = useQuery({
    queryKey: ['escalation-policies'],
    queryFn: () => listEscalationPolicies(),
  });

  const { data: softwareData } = useQuery({
    queryKey: ['software-list'],
    queryFn: () => listSoftware(1, 100),
  });

  const { data: channelsData } = useQuery({
    queryKey: ['notification-channels'],
    queryFn: () => listNotificationChannels(),
  });

  const createMut = useMutation({
    mutationFn: (data: Partial<EscalationPolicy>) => createEscalationPolicy(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['escalation-policies'] });
      setShowCreate(false);
      setNewName('');
      setNewSoftwareId('');
      setNewSeverities([]);
      setNewSteps([{ delay_seconds: 0, channel_id: '', recipients: [] }]);
      addToast({ type: 'success', title: 'Escalation policy created successfully' });
    },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to create escalation policy', message: err?.response?.data?.error || err.message }); },
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => deleteEscalationPolicy(id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['escalation-policies'] }); addToast({ type: 'success', title: 'Escalation policy deleted' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to delete escalation policy', message: err?.response?.data?.error || err.message }); },
  });

  const policies = policiesData?.data ?? [];
  const software = softwareData?.data ?? [];
  const channels = channelsData?.data ?? [];
  const allSeverities = ['critical', 'high', 'medium', 'low'];

  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <p className="text-sm text-gray-500">Define escalation workflows</p>
        <button onClick={() => setShowCreate(true)}
          className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white hover:bg-blue-700">
          <Plus className="h-4 w-4" /> New Policy
        </button>
      </div>

      {isLoading ? (
        <div className="flex h-32 items-center justify-center">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
        </div>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-200 text-left text-xs font-medium uppercase text-gray-500">
                <th className="px-4 py-3">Name</th>
                <th className="px-4 py-3">Software</th>
                <th className="px-4 py-3">Severity Filter</th>
                <th className="px-4 py-3">Steps</th>
                <th className="px-4 py-3">Status</th>
                <th className="px-4 py-3"></th>
              </tr>
            </thead>
            <tbody>
              {policies.length === 0 ? (
                <tr><td colSpan={6} className="px-4 py-8 text-center text-gray-400">No policies defined</td></tr>
              ) : policies.map((p) => {
                const sw = software.find((s) => s.id === p.software_id);
                return (
                  <tr key={p.id} className="border-b border-gray-100 hover:bg-gray-50">
                    <td className="px-4 py-3 font-medium text-gray-900">{p.name}</td>
                    <td className="px-4 py-3 text-gray-600">{sw?.name ?? '--'}</td>
                    <td className="px-4 py-3">
                      <div className="flex flex-wrap gap-1">
                        {p.severity_filter.map((s) => (
                          <span key={s} className={`rounded px-1.5 py-0.5 text-xs font-medium ${
                            s === 'critical' ? 'bg-red-100 text-red-700' :
                            s === 'high' ? 'bg-orange-100 text-orange-700' :
                            s === 'medium' ? 'bg-yellow-100 text-yellow-700' :
                            'bg-blue-100 text-blue-700'
                          }`}>{s}</span>
                        ))}
                      </div>
                    </td>
                    <td className="px-4 py-3 text-gray-600">{p.steps.length}</td>
                    <td className="px-4 py-3">
                      <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                        p.enabled ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'
                      }`}>{p.enabled ? 'Active' : 'Inactive'}</span>
                    </td>
                    <td className="px-4 py-3">
                      <button onClick={() => { if (confirm('Delete policy?')) deleteMut.mutate(p.id); }}
                        className="text-xs text-red-500 hover:text-red-700">Delete</button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {showCreate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-full max-w-2xl max-h-[90vh] overflow-auto rounded-lg bg-white p-6 shadow-xl">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-lg font-semibold text-gray-900">New Escalation Policy</h2>
              <button onClick={() => setShowCreate(false)} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
            </div>
            <form onSubmit={(e) => {
              e.preventDefault();
              createMut.mutate({
                name: newName,
                software_id: newSoftwareId || undefined,
                severity_filter: newSeverities,
                steps: newSteps,
                enabled: true,
              });
            }} className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-gray-700">Name</label>
                <input value={newName} onChange={(e) => setNewName(e.target.value)} required
                  className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none" />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-700">Software</label>
                <select value={newSoftwareId} onChange={(e) => setNewSoftwareId(e.target.value)}
                  className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none">
                  <option value="">All software</option>
                  {software.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
                </select>
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-700 mb-1">Severity Filter</label>
                <div className="flex gap-3">
                  {allSeverities.map((s) => (
                    <label key={s} className="flex items-center gap-1.5 text-sm text-gray-700">
                      <input type="checkbox" checked={newSeverities.includes(s)}
                        onChange={(e) => setNewSeverities(e.target.checked ? [...newSeverities, s] : newSeverities.filter((x) => x !== s))}
                        className="rounded border-gray-300" />
                      <span className="capitalize">{s}</span>
                    </label>
                  ))}
                </div>
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-700 mb-2">Steps</label>
                {newSteps.map((step, i) => (
                  <div key={i} className="mb-3 rounded border border-gray-200 p-3">
                    <div className="flex items-center justify-between mb-2">
                      <span className="text-xs font-medium text-gray-600">Step {i + 1}</span>
                      {newSteps.length > 1 && (
                        <button type="button" onClick={() => setNewSteps(newSteps.filter((_, j) => j !== i))}
                          className="text-xs text-red-500">Remove</button>
                      )}
                    </div>
                    <div className="grid grid-cols-2 gap-3">
                      <div>
                        <label className="block text-xs text-gray-500">Delay (seconds)</label>
                        <input type="number" value={step.delay_seconds}
                          onChange={(e) => { const ns = [...newSteps]; ns[i] = { ...ns[i], delay_seconds: Number(e.target.value) }; setNewSteps(ns); }}
                          className="mt-1 w-full rounded-md border border-gray-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none" />
                      </div>
                      <div>
                        <label className="block text-xs text-gray-500">Channel</label>
                        <select value={step.channel_id}
                          onChange={(e) => { const ns = [...newSteps]; ns[i] = { ...ns[i], channel_id: e.target.value }; setNewSteps(ns); }}
                          className="mt-1 w-full rounded-md border border-gray-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none">
                          <option value="">Select...</option>
                          {channels.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
                        </select>
                      </div>
                    </div>
                    <div className="mt-2">
                      <label className="block text-xs text-gray-500">Recipients (comma separated)</label>
                      <input value={step.recipients.join(', ')}
                        onChange={(e) => { const ns = [...newSteps]; ns[i] = { ...ns[i], recipients: e.target.value.split(',').map((r) => r.trim()).filter(Boolean) }; setNewSteps(ns); }}
                        className="mt-1 w-full rounded-md border border-gray-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none" />
                    </div>
                  </div>
                ))}
                <button type="button" onClick={() => setNewSteps([...newSteps, { delay_seconds: 300, channel_id: '', recipients: [] }])}
                  className="text-xs text-blue-600 hover:text-blue-700">+ Add step</button>
              </div>
              <div className="flex justify-end gap-2">
                <button type="button" onClick={() => setShowCreate(false)}
                  className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
                <button type="submit" disabled={createMut.isPending || !newName.trim()}
                  className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">Create</button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

function LogsTab() {
  const { data: logsData, isLoading } = useQuery({
    queryKey: ['notification-logs'],
    queryFn: () => listNotificationLogs(),
  });

  const logs = logsData?.data ?? [];

  return (
    <div>
      <p className="mb-4 text-sm text-gray-500">Recent notification activity</p>
      {isLoading ? (
        <div className="flex h-32 items-center justify-center">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
        </div>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-200 text-left text-xs font-medium uppercase text-gray-500">
                <th className="px-4 py-3">Event Type</th>
                <th className="px-4 py-3">Incident</th>
                <th className="px-4 py-3">Channel</th>
                <th className="px-4 py-3">Recipient</th>
                <th className="px-4 py-3">Status</th>
                <th className="px-4 py-3">Time</th>
              </tr>
            </thead>
            <tbody>
              {logs.length === 0 ? (
                <tr><td colSpan={6} className="px-4 py-8 text-center text-gray-400">No notifications logged</td></tr>
              ) : logs.map((log) => (
                <tr key={log.id} className="border-b border-gray-100 hover:bg-gray-50">
                  <td className="px-4 py-3 font-medium text-gray-900">{log.event_type}</td>
                  <td className="px-4 py-3">
                    {log.incident_id ? (
                      <a href={`/incidents/${log.incident_id}`} className="inline-flex items-center gap-1 text-blue-600 hover:text-blue-700">
                        <LinkIcon className="h-3 w-3" /> {log.incident_id.slice(0, 8)}
                      </a>
                    ) : '--'}
                  </td>
                  <td className="px-4 py-3 text-gray-600">{log.channel_id?.slice(0, 8) ?? '--'}</td>
                  <td className="px-4 py-3 text-gray-600">{log.recipient ?? '--'}</td>
                  <td className="px-4 py-3">
                    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                      log.status === 'sent' ? 'bg-green-100 text-green-700' :
                      log.status === 'failed' ? 'bg-red-100 text-red-700' :
                      'bg-yellow-100 text-yellow-700'
                    }`}>{log.status}</span>
                  </td>
                  <td className="px-4 py-3 text-xs text-gray-400">
                    {log.sent_at ? new Date(log.sent_at).toLocaleString() : new Date(log.created_at).toLocaleString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

export function NotificationsPage() {
  const [activeTab, setActiveTab] = useState<TabKey>('channels');

  const tabItems: { key: TabKey; label: string }[] = [
    { key: 'channels', label: 'Channels' },
    { key: 'policies', label: 'Escalation Policies' },
    { key: 'logs', label: 'Notification Log' },
  ];

  return (
    <div className="p-8">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Notifications</h1>
        <p className="mt-1 text-sm text-gray-500">Channels, escalation policies, and delivery logs</p>
      </div>

      <div className="mb-6 flex gap-0 border-b border-gray-200">
        {tabItems.map(({ key, label }) => (
          <button
            key={key}
            onClick={() => setActiveTab(key)}
            className={`border-b-2 px-4 py-2.5 text-sm font-medium transition-colors ${
              activeTab === key
                ? 'border-blue-600 text-blue-600'
                : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700'
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {activeTab === 'channels' && <ChannelsTab />}
      {activeTab === 'policies' && <PoliciesTab />}
      {activeTab === 'logs' && <LogsTab />}
    </div>
  );
}
