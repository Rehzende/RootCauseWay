import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Plus, Trash2, Copy, Check } from 'lucide-react';
import { useToast } from '@/components/Toast';
import { listWebhooks, createWebhook, deleteWebhook, listSoftware } from '@/services/api';
import { DataTable, type Column } from '@/components/DataTable';
import type { Webhook, CreateWebhookRequest, WebhookSource } from '@/types/api';

const webhookSources: WebhookSource[] = ['datadog', 'prometheus_alertmanager', 'grafana', 'otel', 'custom'];

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = () => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };
  return (
    <button onClick={handleCopy} className="text-gray-400 hover:text-gray-600" title="Copy">
      {copied ? <Check className="h-3.5 w-3.5 text-green-500" /> : <Copy className="h-3.5 w-3.5" />}
    </button>
  );
}

export function WebhooksPage() {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState<CreateWebhookRequest>({ name: '', source: 'datadog', software_id: '' });

  const { data, isLoading } = useQuery({
    queryKey: ['webhooks'],
    queryFn: listWebhooks,
  });

  const { data: softwareData } = useQuery({
    queryKey: ['software', 'all'],
    queryFn: () => listSoftware(1, 100),
  });

  const createMut = useMutation({
    mutationFn: createWebhook,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['webhooks'] });
      setShowForm(false);
      setForm({ name: '', source: 'datadog', software_id: '' });
      addToast({ type: 'success', title: 'Webhook created successfully' });
    },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to create webhook', message: err?.response?.data?.error || err.message }); },
  });

  const deleteMut = useMutation({
    mutationFn: deleteWebhook,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['webhooks'] }); addToast({ type: 'success', title: 'Webhook deleted' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to delete webhook', message: err?.response?.data?.error || err.message }); },
  });

  const columns: Column<Webhook>[] = [
    { key: 'name', header: 'Name', render: (w) => <span className="font-medium text-gray-900">{w.name}</span> },
    { key: 'source', header: 'Source', render: (w) => (
      <span className="rounded bg-purple-100 px-2 py-0.5 text-xs font-medium text-purple-800">{w.source}</span>
    )},
    { key: 'enabled', header: 'Status', render: (w) => (
      <span className={`text-xs font-medium ${w.enabled ? 'text-green-600' : 'text-gray-400'}`}>
        {w.enabled ? 'Active' : 'Disabled'}
      </span>
    )},
    { key: 'url', header: 'Webhook URL', render: (w) => {
      const url = `${window.location.origin}/api/v1/ingest/${w.endpoint_token}`;
      return (
        <div className="flex items-center gap-2">
          <code className="max-w-xs truncate rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-700">{url}</code>
          <CopyButton text={url} />
        </div>
      );
    }},
    { key: 'actions', header: '', render: (w) => (
      <button onClick={(e) => { e.stopPropagation(); deleteMut.mutate(w.id); }} className="text-red-400 hover:text-red-600">
        <Trash2 className="h-4 w-4" />
      </button>
    )},
  ];

  return (
    <div className="p-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Webhooks</h1>
          <p className="mt-1 text-sm text-gray-500">Manage alert ingestion endpoints</p>
        </div>
        <button
          onClick={() => setShowForm(!showForm)}
          className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
        >
          <Plus className="h-4 w-4" /> Add Webhook
        </button>
      </div>

      {showForm && (
        <div className="mt-4 rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
          <form
            onSubmit={(e) => { e.preventDefault(); createMut.mutate(form); }}
            className="grid grid-cols-1 gap-4 sm:grid-cols-4"
          >
            <input
              placeholder="Webhook Name"
              required
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              className="rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none"
            />
            <select
              value={form.source}
              onChange={(e) => setForm({ ...form, source: e.target.value as WebhookSource })}
              className="rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none"
            >
              {webhookSources.map((s) => (
                <option key={s} value={s}>{s.replace('_', ' ')}</option>
              ))}
            </select>
            <select
              required
              value={form.software_id}
              onChange={(e) => setForm({ ...form, software_id: e.target.value })}
              className="rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none"
            >
              <option value="">Select software...</option>
              {(softwareData?.data ?? []).map((s) => (
                <option key={s.id} value={s.id}>{s.name}</option>
              ))}
            </select>
            <button type="submit" disabled={createMut.isPending} className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">
              {createMut.isPending ? 'Creating...' : 'Create'}
            </button>
          </form>
        </div>
      )}

      <div className="mt-6">
        {isLoading ? (
          <p className="text-sm text-gray-500">Loading...</p>
        ) : (
          <DataTable
            columns={columns}
            data={data?.data ?? []}
            total={data?.total ?? 0}
            page={1}
            perPage={data?.per_page ?? 20}
            onPageChange={() => {}}
            keyExtractor={(w) => w.id}
          />
        )}
      </div>
    </div>
  );
}
