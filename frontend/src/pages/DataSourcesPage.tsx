import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  listObservabilitySources, createObservabilitySource, updateObservabilitySource,
  deleteObservabilitySource, checkSourceHealth, listSnapshotConfigs,
  createSnapshotConfig, updateSnapshotConfig, deleteSnapshotConfig, listSoftware,
} from '@/services/api';
import { PermissionGate } from '@/components/PermissionGate';
import { PermissionButton } from '@/components/PermissionButton';
import { X, Plus, Trash2, RefreshCw, ChevronLeft, Power } from 'lucide-react';
import { useToast } from '@/components/Toast';
import type { ObservabilitySource, ObservabilitySourceType, ObservabilityAuthType, SnapshotConfig, SoftwareEntry } from '@/types/api';

const SOURCE_TYPE_COLORS: Record<string, string> = {
  datadog: '#632CA6',
  prometheus: '#E6522C',
  loki: '#F9D71C',
  tempo: '#1E88E5',
  grafana: '#F46800',
  elasticsearch: '#00BFB3',
  splunk: '#65A637',
  cloudwatch: '#FF9900',
  azure_monitor: '#0078D4',
  gcp_monitoring: '#4285F4',
  newrelic: '#008C99',
  dynatrace: '#1496FF',
  jaeger: '#66CFE3',
  zipkin: '#F44336',
  custom: '#6B7280',
};

const SOURCE_TYPE_LABELS: Record<string, string> = {
  datadog: 'Datadog', prometheus: 'Prometheus', loki: 'Loki', tempo: 'Tempo',
  grafana: 'Grafana', elasticsearch: 'Elasticsearch', splunk: 'Splunk',
  cloudwatch: 'CloudWatch', azure_monitor: 'Azure Monitor', gcp_monitoring: 'GCP Monitoring',
  newrelic: 'New Relic', dynatrace: 'Dynatrace', jaeger: 'Jaeger', zipkin: 'Zipkin', custom: 'Custom',
};

const ALL_SOURCE_TYPES: ObservabilitySourceType[] = [
  'datadog', 'prometheus', 'loki', 'tempo', 'grafana', 'elasticsearch', 'splunk',
  'cloudwatch', 'azure_monitor', 'gcp_monitoring', 'newrelic', 'dynatrace', 'jaeger', 'zipkin', 'custom',
];

const ALL_CAPABILITIES = ['metrics', 'logs', 'traces', 'dashboard', 'alerts'];

function HealthDot({ status }: { status: string }) {
  const color = status === 'healthy' ? 'bg-green-400' : status === 'unhealthy' ? 'bg-red-400' : 'bg-gray-400';
  return <span className={`inline-block h-2.5 w-2.5 rounded-full ${color}`} title={status} />;
}

// --- Create/Edit Modal ---
function SourceModal({ source, onClose }: { source?: ObservabilitySource; onClose: () => void }) {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const isEdit = !!source;

  const [name, setName] = useState(source?.name ?? '');
  const [sourceType, setSourceType] = useState<ObservabilitySourceType>(source?.source_type ?? 'datadog');
  const [baseUrl, setBaseUrl] = useState(source?.base_url ?? '');
  const [description, setDescription] = useState(source?.description ?? '');
  const [environment, setEnvironment] = useState(source?.environment ?? '');
  const [region, setRegion] = useState(source?.region ?? '');
  const [authType, setAuthType] = useState<ObservabilityAuthType>(source?.auth_type ?? 'api_key');
  const [authConfig, setAuthConfig] = useState<Record<string, string>>((source?.auth_config as Record<string, string>) ?? {});
  const [capabilities, setCapabilities] = useState<string[]>(source?.capabilities ?? []);
  const [monitoredSoftwareIds, setMonitoredSoftwareIds] = useState<string[]>(source?.monitored_software_ids ?? []);
  const [timeoutSeconds, setTimeoutSeconds] = useState(source?.timeout_seconds ?? 30);
  const [verifySsl, setVerifySsl] = useState(source?.verify_ssl ?? true);
  const [customHeaders, setCustomHeaders] = useState<Array<{ key: string; value: string }>>(
    Object.entries(source?.custom_headers ?? {}).map(([key, value]) => ({ key, value }))
  );

  const { data: softwareData } = useQuery({ queryKey: ['software'], queryFn: () => listSoftware(1, 200) });
  const softwareList: SoftwareEntry[] = softwareData?.data ?? [];

  const createMut = useMutation({
    mutationFn: () => createObservabilitySource({
      name, source_type: sourceType, base_url: baseUrl, description, environment, region,
      auth_type: authType, auth_config: authConfig, capabilities, monitored_software_ids: monitoredSoftwareIds,
      timeout_seconds: timeoutSeconds, verify_ssl: verifySsl,
      custom_headers: Object.fromEntries(customHeaders.filter((h) => h.key).map((h) => [h.key, h.value])),
    }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['observability-sources'] }); onClose(); addToast({ type: 'success', title: 'Data source created successfully' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to create data source', message: err?.response?.data?.error || err.message }); },
  });

  const updateMut = useMutation({
    mutationFn: () => updateObservabilitySource(source!.id, {
      name, source_type: sourceType, base_url: baseUrl, description, environment, region,
      auth_type: authType, auth_config: authConfig, capabilities, monitored_software_ids: monitoredSoftwareIds,
      timeout_seconds: timeoutSeconds, verify_ssl: verifySsl,
      custom_headers: Object.fromEntries(customHeaders.filter((h) => h.key).map((h) => [h.key, h.value])),
    }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['observability-sources'] }); onClose(); addToast({ type: 'success', title: 'Data source updated successfully' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to update data source', message: err?.response?.data?.error || err.message }); },
  });

  const authFields = () => {
    switch (authType) {
      case 'api_key':
        return (
          <>
            <div>
              <label className="block text-sm font-medium text-gray-700">API Key</label>
              <input type="password" value={authConfig.api_key ?? ''} onChange={(e) => setAuthConfig({ ...authConfig, api_key: e.target.value })}
                className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" placeholder="Enter API key" />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700">Application Key</label>
              <input type="password" value={authConfig.app_key ?? ''} onChange={(e) => setAuthConfig({ ...authConfig, app_key: e.target.value })}
                className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" placeholder="For Datadog (optional)" />
            </div>
          </>
        );
      case 'bearer':
        return (
          <div>
            <label className="block text-sm font-medium text-gray-700">Token</label>
            <input type="password" value={authConfig.token ?? ''} onChange={(e) => setAuthConfig({ ...authConfig, token: e.target.value })}
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" placeholder="Bearer token" />
          </div>
        );
      case 'basic':
        return (
          <>
            <div>
              <label className="block text-sm font-medium text-gray-700">Username</label>
              <input value={authConfig.username ?? ''} onChange={(e) => setAuthConfig({ ...authConfig, username: e.target.value })}
                className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700">Password</label>
              <input type="password" value={authConfig.password ?? ''} onChange={(e) => setAuthConfig({ ...authConfig, password: e.target.value })}
                className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
            </div>
          </>
        );
      case 'oauth2':
        return (
          <>
            <div>
              <label className="block text-sm font-medium text-gray-700">Client ID</label>
              <input value={authConfig.client_id ?? ''} onChange={(e) => setAuthConfig({ ...authConfig, client_id: e.target.value })}
                className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700">Client Secret</label>
              <input type="password" value={authConfig.client_secret ?? ''} onChange={(e) => setAuthConfig({ ...authConfig, client_secret: e.target.value })}
                className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700">Token URL</label>
              <input value={authConfig.token_url ?? ''} onChange={(e) => setAuthConfig({ ...authConfig, token_url: e.target.value })}
                className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" placeholder="https://..." />
            </div>
          </>
        );
      case 'none':
      default:
        return null;
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="max-h-[90vh] w-full max-w-2xl overflow-y-auto rounded-lg bg-white p-6 shadow-xl">
        <div className="mb-4 flex items-center justify-between">
          <h3 className="text-lg font-semibold text-gray-900">{isEdit ? 'Edit' : 'Add'} Data Source</h3>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
        </div>
        <form onSubmit={(e) => { e.preventDefault(); isEdit ? updateMut.mutate() : createMut.mutate(); }} className="space-y-6">
          {/* Basic */}
          <fieldset className="space-y-3">
            <legend className="text-sm font-semibold text-gray-800">Basic</legend>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-sm font-medium text-gray-700">Name</label>
                <input value={name} onChange={(e) => setName(e.target.value)} required className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Source Type</label>
                <select value={sourceType} onChange={(e) => setSourceType(e.target.value as ObservabilitySourceType)} className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm">
                  {ALL_SOURCE_TYPES.map((t) => <option key={t} value={t}>{SOURCE_TYPE_LABELS[t]}</option>)}
                </select>
              </div>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700">Base URL</label>
              <input value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} required placeholder="https://..." className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700">Description</label>
              <input value={description} onChange={(e) => setDescription(e.target.value)} className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-sm font-medium text-gray-700">Environment</label>
                <input value={environment} onChange={(e) => setEnvironment(e.target.value)} placeholder="production" className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Region</label>
                <input value={region} onChange={(e) => setRegion(e.target.value)} placeholder="us-east-1" className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
              </div>
            </div>
          </fieldset>

          {/* Authentication */}
          <fieldset className="space-y-3">
            <legend className="text-sm font-semibold text-gray-800">Authentication</legend>
            <div>
              <label className="block text-sm font-medium text-gray-700">Auth Type</label>
              <select value={authType} onChange={(e) => { setAuthType(e.target.value as ObservabilityAuthType); setAuthConfig({}); }}
                className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm">
                <option value="api_key">API Key</option>
                <option value="bearer">Bearer Token</option>
                <option value="basic">Basic Auth</option>
                <option value="oauth2">OAuth2</option>
                <option value="none">None</option>
              </select>
            </div>
            {authFields()}
          </fieldset>

          {/* Monitoring */}
          <fieldset className="space-y-3">
            <legend className="text-sm font-semibold text-gray-800">Monitoring</legend>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Capabilities</label>
              <div className="flex flex-wrap gap-2">
                {ALL_CAPABILITIES.map((cap) => (
                  <label key={cap} className="flex items-center gap-1.5 text-sm">
                    <input type="checkbox" checked={capabilities.includes(cap)}
                      onChange={(e) => setCapabilities(e.target.checked ? [...capabilities, cap] : capabilities.filter((c) => c !== cap))}
                      className="rounded border-gray-300" />
                    {cap}
                  </label>
                ))}
              </div>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Monitored Software</label>
              <div className="max-h-32 overflow-y-auto rounded-md border border-gray-200 p-2 space-y-1">
                {softwareList.map((sw) => (
                  <label key={sw.id} className="flex items-center gap-1.5 text-sm">
                    <input type="checkbox" checked={monitoredSoftwareIds.includes(sw.id)}
                      onChange={(e) => setMonitoredSoftwareIds(e.target.checked ? [...monitoredSoftwareIds, sw.id] : monitoredSoftwareIds.filter((id) => id !== sw.id))}
                      className="rounded border-gray-300" />
                    {sw.name}
                  </label>
                ))}
                {softwareList.length === 0 && <p className="text-xs text-gray-400">No software in catalog</p>}
              </div>
            </div>
          </fieldset>

          {/* Advanced */}
          <fieldset className="space-y-3">
            <legend className="text-sm font-semibold text-gray-800">Advanced</legend>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-sm font-medium text-gray-700">Timeout (seconds)</label>
                <input type="number" value={timeoutSeconds} onChange={(e) => setTimeoutSeconds(Number(e.target.value))} min={1} max={300}
                  className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
              </div>
              <div className="flex items-end pb-2">
                <label className="flex items-center gap-2 text-sm">
                  <input type="checkbox" checked={verifySsl} onChange={(e) => setVerifySsl(e.target.checked)} className="rounded border-gray-300" />
                  Verify SSL
                </label>
              </div>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Custom Headers</label>
              {customHeaders.map((h, i) => (
                <div key={i} className="mb-1 flex gap-2">
                  <input value={h.key} onChange={(e) => { const nh = [...customHeaders]; nh[i] = { ...nh[i], key: e.target.value }; setCustomHeaders(nh); }}
                    placeholder="Header name" className="flex-1 rounded-md border border-gray-300 px-2 py-1 text-sm" />
                  <input value={h.value} onChange={(e) => { const nh = [...customHeaders]; nh[i] = { ...nh[i], value: e.target.value }; setCustomHeaders(nh); }}
                    placeholder="Value" className="flex-1 rounded-md border border-gray-300 px-2 py-1 text-sm" />
                  <button type="button" onClick={() => setCustomHeaders(customHeaders.filter((_, j) => j !== i))} className="text-gray-400 hover:text-red-500">
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              ))}
              <button type="button" onClick={() => setCustomHeaders([...customHeaders, { key: '', value: '' }])}
                className="text-xs text-blue-600 hover:text-blue-800">+ Add header</button>
            </div>
          </fieldset>

          <div className="flex justify-end gap-2 border-t border-gray-200 pt-4">
            <button type="button" onClick={onClose} className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
            <button type="submit" disabled={createMut.isPending || updateMut.isPending}
              className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">
              {(createMut.isPending || updateMut.isPending) ? 'Saving...' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// --- Snapshot Config Modal ---
function SnapshotModal({ sourceId, config, onClose }: { sourceId: string; config?: SnapshotConfig; onClose: () => void }) {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const isEdit = !!config;
  const [name, setName] = useState(config?.name ?? '');
  const [snapshotType, setSnapshotType] = useState(config?.snapshot_type ?? 'metrics');
  const [queryTemplate, setQueryTemplate] = useState(config?.query_template ?? '');
  const [timeRange, setTimeRange] = useState(config?.time_range_seconds ?? 3600);
  const [enabled, setEnabled] = useState(config?.enabled ?? true);

  const createMut = useMutation({
    mutationFn: () => createSnapshotConfig(sourceId, { name, snapshot_type: snapshotType, query_template: queryTemplate, time_range_seconds: timeRange, enabled, parameters: {} }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['snapshot-configs', sourceId] }); onClose(); addToast({ type: 'success', title: 'Snapshot config created successfully' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to create snapshot config', message: err?.response?.data?.error || err.message }); },
  });
  const updateMut = useMutation({
    mutationFn: () => updateSnapshotConfig(sourceId, config!.id, { name, snapshot_type: snapshotType, query_template: queryTemplate, time_range_seconds: timeRange, enabled }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['snapshot-configs', sourceId] }); onClose(); addToast({ type: 'success', title: 'Snapshot config updated successfully' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to update snapshot config', message: err?.response?.data?.error || err.message }); },
  });

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-md rounded-lg bg-white p-6 shadow-xl">
        <div className="mb-4 flex items-center justify-between">
          <h3 className="text-lg font-semibold text-gray-900">{isEdit ? 'Edit' : 'Add'} Snapshot Config</h3>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
        </div>
        <form onSubmit={(e) => { e.preventDefault(); isEdit ? updateMut.mutate() : createMut.mutate(); }} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700">Name</label>
            <input value={name} onChange={(e) => setName(e.target.value)} required className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Type</label>
            <select value={snapshotType} onChange={(e) => setSnapshotType(e.target.value as SnapshotConfig['snapshot_type'])}
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm">
              {['metrics', 'logs', 'traces', 'dashboard', 'alerts', 'custom'].map((t) => <option key={t} value={t}>{t}</option>)}
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Query Template</label>
            <textarea value={queryTemplate} onChange={(e) => setQueryTemplate(e.target.value)} rows={4}
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 font-mono text-sm" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Time Range (seconds)</label>
            <input type="number" value={timeRange} onChange={(e) => setTimeRange(Number(e.target.value))} min={60}
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
          </div>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} className="rounded border-gray-300" />
            Enabled
          </label>
          <div className="flex justify-end gap-2">
            <button type="button" onClick={onClose} className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
            <button type="submit" disabled={createMut.isPending || updateMut.isPending}
              className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">
              {(createMut.isPending || updateMut.isPending) ? 'Saving...' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// --- Source Detail View ---
function SourceDetailView({ source, onBack }: { source: ObservabilitySource; onBack: () => void }) {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [showSnapshotModal, setShowSnapshotModal] = useState(false);
  const [editSnapshot, setEditSnapshot] = useState<SnapshotConfig | undefined>();

  const { data: snapshots } = useQuery({
    queryKey: ['snapshot-configs', source.id],
    queryFn: () => listSnapshotConfigs(source.id),
  });

  const healthMut = useMutation({
    mutationFn: () => checkSourceHealth(source.id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['observability-sources'] }); addToast({ type: 'success', title: 'Connection test passed' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Connection test failed', message: err?.response?.data?.error || err.message }); },
  });

  const deleteSnapMut = useMutation({
    mutationFn: (configId: string) => deleteSnapshotConfig(source.id, configId),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['snapshot-configs', source.id] }); addToast({ type: 'success', title: 'Snapshot config deleted' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to delete snapshot config', message: err?.response?.data?.error || err.message }); },
  });

  const color = SOURCE_TYPE_COLORS[source.source_type] ?? SOURCE_TYPE_COLORS.custom;

  const SNAPSHOT_TYPE_COLORS: Record<string, string> = {
    metrics: 'bg-blue-100 text-blue-700', logs: 'bg-yellow-100 text-yellow-700',
    traces: 'bg-purple-100 text-purple-700', dashboard: 'bg-green-100 text-green-700',
    alerts: 'bg-red-100 text-red-700', custom: 'bg-gray-100 text-gray-700',
  };

  return (
    <div>
      <button onClick={onBack} className="mb-4 flex items-center gap-1 text-sm text-gray-500 hover:text-gray-700">
        <ChevronLeft className="h-4 w-4" /> Back to sources
      </button>

      <div className="rounded-lg border bg-white shadow-sm" style={{ borderLeftColor: color, borderLeftWidth: 4 }}>
        <div className="p-6">
          <div className="flex items-start justify-between">
            <div>
              <div className="flex items-center gap-3">
                <h2 className="text-xl font-semibold text-gray-900">{source.name}</h2>
                <HealthDot status={source.health_status} />
                <span className="rounded-full px-2 py-0.5 text-xs font-medium text-white" style={{ backgroundColor: color }}>
                  {SOURCE_TYPE_LABELS[source.source_type]}
                </span>
              </div>
              {source.description && <p className="mt-1 text-sm text-gray-500">{source.description}</p>}
              <p className="mt-2 font-mono text-sm text-gray-600">{source.base_url}</p>
            </div>
            <button onClick={() => healthMut.mutate()} disabled={healthMut.isPending}
              className="flex items-center gap-1.5 rounded-md border border-gray-300 px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50">
              <RefreshCw className={`h-4 w-4 ${healthMut.isPending ? 'animate-spin' : ''}`} /> Test Connection
            </button>
          </div>

          <div className="mt-4 flex flex-wrap gap-4 text-sm text-gray-600">
            <span>Auth: <strong>{source.auth_type}</strong></span>
            <span>Timeout: <strong>{source.timeout_seconds}s</strong></span>
            <span>SSL: <strong>{source.verify_ssl ? 'Yes' : 'No'}</strong></span>
            {source.environment && <span>Env: <strong>{source.environment}</strong></span>}
            {source.region && <span>Region: <strong>{source.region}</strong></span>}
          </div>

          {source.capabilities.length > 0 && (
            <div className="mt-3 flex flex-wrap gap-1">
              {source.capabilities.map((cap) => (
                <span key={cap} className="rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700">{cap}</span>
              ))}
            </div>
          )}

          {source.last_health_check && (
            <p className="mt-3 text-xs text-gray-400">Last health check: {new Date(source.last_health_check).toLocaleString()}</p>
          )}
        </div>
      </div>

      {/* Snapshot Configs */}
      <div className="mt-6">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-semibold text-gray-900">Snapshot Configs</h3>
          <PermissionGate resource="observability" action="write">
            <button onClick={() => { setEditSnapshot(undefined); setShowSnapshotModal(true); }}
              className="flex items-center gap-1.5 rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700">
              <Plus className="h-4 w-4" /> Add Config
            </button>
          </PermissionGate>
        </div>
        <div className="mt-3 space-y-3">
          {(snapshots ?? []).length === 0 && <p className="text-sm text-gray-400">No snapshot configs yet</p>}
          {(snapshots ?? []).map((cfg) => (
            <div key={cfg.id} className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
              <div className="flex items-start justify-between">
                <div>
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-gray-900">{cfg.name}</span>
                    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${SNAPSHOT_TYPE_COLORS[cfg.snapshot_type] ?? SNAPSHOT_TYPE_COLORS.custom}`}>
                      {cfg.snapshot_type}
                    </span>
                    {!cfg.enabled && <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-500">disabled</span>}
                  </div>
                  <p className="mt-1 text-xs text-gray-500">Time range: {cfg.time_range_seconds}s</p>
                </div>
                <div className="flex gap-2">
                  <PermissionButton resource="observability" action="write"
                    onClick={() => { setEditSnapshot(cfg); setShowSnapshotModal(true); }}
                    className="text-sm text-blue-600 hover:text-blue-800">Edit</PermissionButton>
                  <PermissionGate resource="observability" action="write">
                    <button onClick={() => { if (confirm('Delete this snapshot config?')) deleteSnapMut.mutate(cfg.id); }}
                      className="text-gray-400 hover:text-red-600"><Trash2 className="h-4 w-4" /></button>
                  </PermissionGate>
                </div>
              </div>
              {cfg.query_template && (
                <pre className="mt-2 overflow-x-auto rounded-md bg-gray-900 p-3 text-xs text-green-400">{cfg.query_template}</pre>
              )}
            </div>
          ))}
        </div>
      </div>

      {showSnapshotModal && (
        <SnapshotModal sourceId={source.id} config={editSnapshot} onClose={() => setShowSnapshotModal(false)} />
      )}
    </div>
  );
}

// --- Main Page ---
export function DataSourcesPage() {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [showModal, setShowModal] = useState(false);
  const [editSource, setEditSource] = useState<ObservabilitySource | undefined>();
  const [selectedSource, setSelectedSource] = useState<ObservabilitySource | null>(null);

  const { data: sources, isLoading } = useQuery({
    queryKey: ['observability-sources'],
    queryFn: () => listObservabilitySources(),
  });

  const deleteMut = useMutation({
    mutationFn: deleteObservabilitySource,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['observability-sources'] }); addToast({ type: 'success', title: 'Data source deleted' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to delete data source', message: err?.response?.data?.error || err.message }); },
  });

  const toggleMut = useMutation({
    mutationFn: (s: ObservabilitySource) => updateObservabilitySource(s.id, { enabled: !s.enabled }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['observability-sources'] }); addToast({ type: 'success', title: 'Data source status updated' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to update data source', message: err?.response?.data?.error || err.message }); },
  });

  const healthMut = useMutation({
    mutationFn: checkSourceHealth,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['observability-sources'] }); addToast({ type: 'success', title: 'Connection test passed' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Connection test failed', message: err?.response?.data?.error || err.message }); },
  });

  if (selectedSource) {
    return (
      <div className="p-8">
        <SourceDetailView source={selectedSource} onBack={() => setSelectedSource(null)} />
      </div>
    );
  }

  // Group sources by type
  const grouped: Record<string, ObservabilitySource[]> = {};
  for (const s of sources ?? []) {
    (grouped[s.source_type] ??= []).push(s);
  }

  return (
    <div className="p-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Data Sources</h1>
          <p className="mt-1 text-sm text-gray-500">Connect observability platforms to collect metrics, logs, and traces</p>
        </div>
        <PermissionGate resource="observability" action="write">
          <button onClick={() => { setEditSource(undefined); setShowModal(true); }}
            className="flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700">
            <Plus className="h-4 w-4" /> Add Data Source
          </button>
        </PermissionGate>
      </div>

      {isLoading ? (
        <div className="mt-8 text-sm text-gray-500">Loading data sources...</div>
      ) : (sources ?? []).length === 0 ? (
        <div className="mt-12 text-center">
          <p className="text-sm text-gray-400">No data sources configured yet</p>
          <p className="mt-1 text-xs text-gray-400">Add a data source to start collecting observability data</p>
        </div>
      ) : (
        <div className="mt-6 space-y-8">
          {Object.entries(grouped).map(([type, items]) => {
            const color = SOURCE_TYPE_COLORS[type] ?? SOURCE_TYPE_COLORS.custom;
            return (
              <div key={type}>
                <div className="mb-3 flex items-center gap-2">
                  <span className="inline-block h-3 w-3 rounded-sm" style={{ backgroundColor: color }} />
                  <h2 className="text-sm font-semibold uppercase tracking-wider text-gray-600">
                    {SOURCE_TYPE_LABELS[type] ?? type} ({items.length})
                  </h2>
                </div>
                <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                  {items.map((s) => (
                    <div key={s.id} className="group cursor-pointer rounded-lg border border-gray-200 bg-white shadow-sm transition-shadow hover:shadow-md"
                      style={{ borderLeftColor: color, borderLeftWidth: 4 }}
                      onClick={() => setSelectedSource(s)}>
                      <div className="p-4">
                        <div className="flex items-start justify-between">
                          <div className="flex items-center gap-2">
                            <HealthDot status={s.health_status} />
                            <h3 className="font-medium text-gray-900">{s.name}</h3>
                          </div>
                          <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
                            <button onClick={() => toggleMut.mutate(s)} title={s.enabled ? 'Disable' : 'Enable'}
                              className={`rounded p-1 ${s.enabled ? 'text-green-600 hover:bg-green-50' : 'text-gray-400 hover:bg-gray-50'}`}>
                              <Power className="h-4 w-4" />
                            </button>
                            <button onClick={() => healthMut.mutate(s.id)} title="Test Connection"
                              className="rounded p-1 text-gray-400 hover:bg-gray-50 hover:text-blue-600">
                              <RefreshCw className={`h-4 w-4 ${healthMut.isPending ? 'animate-spin' : ''}`} />
                            </button>
                          </div>
                        </div>
                        <p className="mt-1 truncate font-mono text-xs text-gray-500">{s.base_url}</p>
                        {s.capabilities.length > 0 && (
                          <div className="mt-2 flex flex-wrap gap-1">
                            {s.capabilities.map((cap) => (
                              <span key={cap} className="rounded-full bg-blue-50 px-1.5 py-0.5 text-[10px] font-medium text-blue-600">{cap}</span>
                            ))}
                          </div>
                        )}
                        <div className="mt-2 flex flex-wrap gap-1">
                          {s.environment && (
                            <span className="rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-600">{s.environment}</span>
                          )}
                          {s.region && (
                            <span className="rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-600">{s.region}</span>
                          )}
                        </div>
                      </div>
                      <div className="flex items-center justify-between border-t border-gray-100 px-4 py-2" onClick={(e) => e.stopPropagation()}>
                        <PermissionButton resource="observability" action="write"
                          onClick={() => { setEditSource(s); setShowModal(true); }}
                          className="text-xs text-blue-600 hover:text-blue-800">Edit</PermissionButton>
                        <PermissionGate resource="observability" action="write">
                          <button onClick={() => { if (confirm('Delete this data source?')) deleteMut.mutate(s.id); }}
                            className="text-gray-400 hover:text-red-600"><Trash2 className="h-3.5 w-3.5" /></button>
                        </PermissionGate>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            );
          })}
        </div>
      )}

      {showModal && <SourceModal source={editSource} onClose={() => setShowModal(false)} />}
    </div>
  );
}
