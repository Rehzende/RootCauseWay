import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { useMutation } from '@tanstack/react-query';
import {
  CheckCircle, ArrowRight, ArrowLeft, Copy, Check, Zap,
  Webhook, AlertTriangle, Radio, BarChart3, Activity, Loader2,
  SkipForward,
} from 'lucide-react';
import { useToast } from '@/components/Toast';
import api, { createSoftware, createWebhook } from '@/services/api';
import type { CloudProvider, WebhookSource } from '@/types/api';

const STEPS = ['Welcome', 'Add Service', 'Alert Source', 'Verify', 'Done'];

interface AlertSource {
  id: WebhookSource;
  name: string;
  icon: React.ComponentType<{ className?: string }>;
  color: string;
  bgColor: string;
  instructions: string;
}

const ALERT_SOURCES: AlertSource[] = [
  {
    id: 'prometheus_alertmanager',
    name: 'Prometheus AlertManager',
    icon: Activity,
    color: 'text-orange-600',
    bgColor: 'bg-orange-50 border-orange-200',
    instructions: 'Add this webhook URL to your alertmanager.yml under receivers as a webhook_configs entry.',
  },
  {
    id: 'datadog',
    name: 'Datadog',
    icon: BarChart3,
    color: 'text-purple-600',
    bgColor: 'bg-purple-50 border-purple-200',
    instructions: 'Go to Datadog Settings > Integrations > Webhooks. Create a new webhook integration using this URL.',
  },
  {
    id: 'grafana',
    name: 'Grafana',
    icon: Radio,
    color: 'text-amber-600',
    bgColor: 'bg-amber-50 border-amber-200',
    instructions: 'In Grafana, go to Alerting > Contact Points. Add a new contact point of type "Webhook" with this URL.',
  },
  {
    id: 'otel',
    name: 'OpenTelemetry',
    icon: Webhook,
    color: 'text-blue-600',
    bgColor: 'bg-blue-50 border-blue-200',
    instructions: 'Configure an OTLP HTTP exporter in your collector config to send alerts to this webhook URL.',
  },
];

export function OnboardingPage() {
  const navigate = useNavigate();
  const { addToast } = useToast();
  const [step, setStep] = useState(0);
  const [direction, setDirection] = useState<'forward' | 'back'>('forward');

  // Step 2: Add Service
  const [softwareName, setSoftwareName] = useState('');
  const [softwareSlug, setSoftwareSlug] = useState('');
  const [softwareDesc, setSoftwareDesc] = useState('');
  const [cloudProvider, setCloudProvider] = useState<CloudProvider>('aws');
  const [repoUrl, setRepoUrl] = useState('');
  const [tagsInput, setTagsInput] = useState('');
  const [softwareId, setSoftwareId] = useState('');

  // Step 3: Alert Source
  const [selectedSource, setSelectedSource] = useState<WebhookSource | null>(null);
  const [webhookToken, setWebhookToken] = useState('');
  const [webhookUrl, setWebhookUrl] = useState('');
  const [copied, setCopied] = useState(false);

  // Step 4: Verify
  const [testStatus, setTestStatus] = useState<'idle' | 'sending' | 'success' | 'error'>('idle');

  const goTo = useCallback((target: number) => {
    setDirection(target > step ? 'forward' : 'back');
    setStep(target);
  }, [step]);

  const cloudProviders: { value: CloudProvider; label: string }[] = [
    { value: 'aws', label: 'AWS' },
    { value: 'gcp', label: 'GCP' },
    { value: 'azure', label: 'Azure' },
    { value: 'on_prem', label: 'On-Prem' },
    { value: 'hybrid', label: 'Other' },
  ];

  // Auto-generate slug from name
  useEffect(() => {
    const autoSlug = softwareName
      .toLowerCase()
      .replace(/[^a-z0-9\s-]/g, '')
      .replace(/\s+/g, '-')
      .replace(/-+/g, '-')
      .replace(/^-|-$/g, '');
    setSoftwareSlug(autoSlug);
  }, [softwareName]);

  const createSoftwareMut = useMutation({
    mutationFn: () => {
      const tags = tagsInput
        .split(',')
        .map((t) => t.trim())
        .filter(Boolean);
      return createSoftware({
        name: softwareName,
        slug: softwareSlug,
        description: softwareDesc || undefined,
        cloud_provider: cloudProvider,
        repository_url: repoUrl || undefined,
        tags: tags.length > 0 ? tags : undefined,
      });
    },
    onSuccess: (data) => {
      setSoftwareId(data.id);
      goTo(2);
      addToast({ type: 'success', title: 'Service created successfully' });
    },
    onError: (err: any) => {
      addToast({
        type: 'error',
        title: 'Failed to create service',
        message: err?.response?.data?.error || err.message,
      });
    },
  });

  const createWebhookMut = useMutation({
    mutationFn: () =>
      createWebhook({
        name: `${softwareName} - ${ALERT_SOURCES.find((s) => s.id === selectedSource)?.name ?? 'Webhook'}`,
        software_id: softwareId,
        source: selectedSource!,
      }),
    onSuccess: (data) => {
      setWebhookToken(data.endpoint_token);
      setWebhookUrl(`${window.location.origin}/api/v1/ingest/${data.endpoint_token}`);
      addToast({ type: 'success', title: 'Webhook created' });
    },
    onError: (err: any) => {
      addToast({
        type: 'error',
        title: 'Failed to create webhook',
        message: err?.response?.data?.error || err.message,
      });
    },
  });

  const handleSelectSource = (source: WebhookSource) => {
    setSelectedSource(source);
    setWebhookToken('');
    setWebhookUrl('');
  };

  const handleCopy = () => {
    navigator.clipboard.writeText(webhookUrl);
    setCopied(true);
    addToast({ type: 'success', title: 'Copied to clipboard' });
    setTimeout(() => setCopied(false), 2000);
  };

  const handleSendTestAlert = async () => {
    if (!webhookToken) return;
    setTestStatus('sending');
    try {
      await api.post(`/ingest/${webhookToken}`, {
        alerts: [
          {
            status: 'firing',
            labels: {
              alertname: 'OnboardingTest',
              severity: 'low',
              service: softwareSlug,
            },
            annotations: {
              summary: 'Test alert from RootCauseway onboarding wizard',
            },
            startsAt: new Date().toISOString(),
          },
        ],
      });
      setTestStatus('success');
      addToast({ type: 'success', title: 'Test alert sent successfully' });
    } catch {
      setTestStatus('error');
      addToast({ type: 'error', title: 'Failed to send test alert' });
    }
  };

  const selectedSourceInfo = ALERT_SOURCES.find((s) => s.id === selectedSource);

  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-gray-50 to-gray-100 p-6">
      <div className="w-full max-w-2xl">
        {/* Step indicator */}
        <div className="mb-8 flex items-center justify-center gap-0">
          {STEPS.map((label, i) => (
            <div key={label} className="flex items-center">
              <div className="flex flex-col items-center">
                <div
                  className={`flex h-9 w-9 items-center justify-center rounded-full text-xs font-bold transition-all duration-300 ${
                    i < step
                      ? 'bg-green-500 text-white shadow-sm shadow-green-200'
                      : i === step
                        ? 'bg-blue-600 text-white shadow-md shadow-blue-200 scale-110'
                        : 'bg-gray-200 text-gray-400'
                  }`}
                >
                  {i < step ? <Check className="h-4 w-4" /> : i + 1}
                </div>
                <span
                  className={`mt-1.5 text-[10px] font-medium w-20 text-center transition-colors ${
                    i <= step ? 'text-gray-700' : 'text-gray-400'
                  }`}
                >
                  {label}
                </span>
              </div>
              {i < STEPS.length - 1 && (
                <div
                  className={`mx-1 h-0.5 w-12 transition-colors duration-500 ${
                    i < step ? 'bg-green-500' : 'bg-gray-200'
                  }`}
                />
              )}
            </div>
          ))}
        </div>

        {/* Card */}
        <div
          key={step}
          className={`rounded-xl border border-gray-200 bg-white p-8 shadow-sm transition-all duration-300 ${
            direction === 'forward' ? 'animate-fade-in-right' : 'animate-fade-in-left'
          }`}
        >
          {/* Step 0: Welcome */}
          {step === 0 && (
            <div className="text-center">
              <div className="mx-auto mb-5 flex h-20 w-20 items-center justify-center rounded-2xl bg-gradient-to-br from-blue-500 to-indigo-600 shadow-lg shadow-blue-200">
                <AlertTriangle className="h-10 w-10 text-white" />
              </div>
              <h1 className="text-3xl font-bold text-gray-900">Welcome to RootCauseway</h1>
              <p className="mx-auto mt-4 max-w-md text-gray-500 leading-relaxed">
                Let's set up your incident intelligence platform in a few steps.
                You'll register a service, connect an alert source, and be ready to go.
              </p>
              <button
                onClick={() => goTo(1)}
                className="mt-8 inline-flex items-center gap-2 rounded-lg bg-blue-600 px-8 py-3 text-sm font-semibold text-white shadow-sm hover:bg-blue-700 hover:shadow-md transition-all"
              >
                Get Started <ArrowRight className="h-4 w-4" />
              </button>
            </div>
          )}

          {/* Step 1: Add Service */}
          {step === 1 && (
            <div>
              <div className="mb-6">
                <h2 className="text-xl font-bold text-gray-900">Add Your First Service</h2>
                <p className="mt-1 text-sm text-gray-500">
                  Register the software service you want to monitor for incidents.
                </p>
              </div>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700">
                    Name <span className="text-red-500">*</span>
                  </label>
                  <input
                    value={softwareName}
                    onChange={(e) => setSoftwareName(e.target.value)}
                    placeholder="e.g. API Gateway"
                    className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none transition-colors"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700">Slug</label>
                  <input
                    value={softwareSlug}
                    onChange={(e) => setSoftwareSlug(e.target.value)}
                    placeholder="api-gateway"
                    className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm font-mono focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none transition-colors"
                  />
                  <p className="mt-1 text-xs text-gray-400">Auto-generated from name. You can edit it.</p>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700">Description</label>
                  <textarea
                    value={softwareDesc}
                    onChange={(e) => setSoftwareDesc(e.target.value)}
                    placeholder="Brief description of this service..."
                    rows={2}
                    className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none transition-colors resize-none"
                  />
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700">Cloud Provider</label>
                    <select
                      value={cloudProvider}
                      onChange={(e) => setCloudProvider(e.target.value as CloudProvider)}
                      className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none transition-colors"
                    >
                      {cloudProviders.map((p) => (
                        <option key={p.value} value={p.value}>
                          {p.label}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700">Repository URL</label>
                    <input
                      value={repoUrl}
                      onChange={(e) => setRepoUrl(e.target.value)}
                      placeholder="https://github.com/..."
                      className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none transition-colors"
                    />
                  </div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700">Tags</label>
                  <input
                    value={tagsInput}
                    onChange={(e) => setTagsInput(e.target.value)}
                    placeholder="backend, payments, critical (comma-separated)"
                    className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none transition-colors"
                  />
                </div>
              </div>
              <div className="mt-6 flex justify-between">
                <button
                  onClick={() => goTo(0)}
                  className="inline-flex items-center gap-1 rounded-lg border border-gray-300 px-4 py-2.5 text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors"
                >
                  <ArrowLeft className="h-4 w-4" /> Back
                </button>
                <button
                  onClick={() => createSoftwareMut.mutate()}
                  disabled={!softwareName || !softwareSlug || createSoftwareMut.isPending}
                  className="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-5 py-2.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  {createSoftwareMut.isPending ? (
                    <>
                      <Loader2 className="h-4 w-4 animate-spin" /> Creating...
                    </>
                  ) : (
                    <>
                      Next <ArrowRight className="h-4 w-4" />
                    </>
                  )}
                </button>
              </div>
            </div>
          )}

          {/* Step 2: Connect Alert Source */}
          {step === 2 && (
            <div>
              <div className="mb-6">
                <h2 className="text-xl font-bold text-gray-900">Connect Alert Source</h2>
                <p className="mt-1 text-sm text-gray-500">
                  Choose where your alerts come from. We'll generate a webhook URL for you.
                </p>
              </div>

              {/* Source cards */}
              <div className="grid grid-cols-2 gap-3 mb-6">
                {ALERT_SOURCES.map((source) => {
                  const Icon = source.icon;
                  const isSelected = selectedSource === source.id;
                  return (
                    <button
                      key={source.id}
                      onClick={() => handleSelectSource(source.id)}
                      className={`flex flex-col items-center gap-2 rounded-xl border-2 p-4 text-center transition-all hover:shadow-sm ${
                        isSelected
                          ? `${source.bgColor} border-current ${source.color} shadow-sm`
                          : 'border-gray-200 hover:border-gray-300 text-gray-600'
                      }`}
                    >
                      <Icon className={`h-7 w-7 ${isSelected ? source.color : 'text-gray-400'}`} />
                      <span className="text-sm font-medium">{source.name}</span>
                    </button>
                  );
                })}
              </div>

              {/* Create webhook button */}
              {selectedSource && !webhookUrl && (
                <div className="text-center py-2">
                  <button
                    onClick={() => createWebhookMut.mutate()}
                    disabled={createWebhookMut.isPending}
                    className="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-5 py-2.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50 transition-colors"
                  >
                    {createWebhookMut.isPending ? (
                      <>
                        <Loader2 className="h-4 w-4 animate-spin" /> Creating webhook...
                      </>
                    ) : (
                      <>
                        <Webhook className="h-4 w-4" /> Generate Webhook
                      </>
                    )}
                  </button>
                </div>
              )}

              {/* Webhook URL + instructions */}
              {webhookUrl && selectedSourceInfo && (
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1.5">
                      Your Webhook URL
                    </label>
                    <div className="flex items-center gap-2">
                      <code className="flex-1 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2.5 text-xs text-gray-700 font-mono truncate select-all">
                        {webhookUrl}
                      </code>
                      <button
                        onClick={handleCopy}
                        className="flex-shrink-0 rounded-lg border border-gray-300 p-2.5 text-gray-500 hover:bg-gray-50 hover:text-gray-700 transition-colors"
                        title="Copy URL"
                      >
                        {copied ? (
                          <Check className="h-4 w-4 text-green-600" />
                        ) : (
                          <Copy className="h-4 w-4" />
                        )}
                      </button>
                    </div>
                  </div>

                  <div className={`rounded-lg border p-4 ${selectedSourceInfo.bgColor}`}>
                    <p className={`text-sm font-semibold ${selectedSourceInfo.color}`}>
                      {selectedSourceInfo.name} Integration
                    </p>
                    <p className="mt-1 text-sm text-gray-600">{selectedSourceInfo.instructions}</p>
                  </div>
                </div>
              )}

              <div className="mt-6 flex justify-between">
                <button
                  onClick={() => goTo(1)}
                  className="inline-flex items-center gap-1 rounded-lg border border-gray-300 px-4 py-2.5 text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors"
                >
                  <ArrowLeft className="h-4 w-4" /> Back
                </button>
                <button
                  onClick={() => goTo(3)}
                  disabled={!webhookUrl}
                  className="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-5 py-2.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  Next <ArrowRight className="h-4 w-4" />
                </button>
              </div>
            </div>
          )}

          {/* Step 3: Verify Connection */}
          {step === 3 && (
            <div>
              <div className="mb-6">
                <h2 className="text-xl font-bold text-gray-900">Verify Connection</h2>
                <p className="mt-1 text-sm text-gray-500">
                  Send a test alert to make sure everything is wired up correctly.
                </p>
              </div>

              <div className="flex flex-col items-center py-8">
                {testStatus === 'idle' && (
                  <>
                    <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-amber-50">
                      <Zap className="h-8 w-8 text-amber-500" />
                    </div>
                    <p className="mb-6 text-sm text-gray-500 text-center max-w-sm">
                      Click the button below to send a test alert through your webhook.
                      This will create a test incident in RootCauseway.
                    </p>
                    <button
                      onClick={handleSendTestAlert}
                      className="inline-flex items-center gap-2 rounded-lg bg-amber-500 px-6 py-3 text-sm font-semibold text-white hover:bg-amber-600 shadow-sm transition-all"
                    >
                      <Zap className="h-4 w-4" /> Send Test Alert
                    </button>
                  </>
                )}

                {testStatus === 'sending' && (
                  <>
                    <Loader2 className="h-12 w-12 text-blue-500 animate-spin mb-4" />
                    <p className="text-sm text-gray-500">Sending test alert...</p>
                  </>
                )}

                {testStatus === 'success' && (
                  <>
                    <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-green-50">
                      <CheckCircle className="h-10 w-10 text-green-500" />
                    </div>
                    <p className="text-lg font-semibold text-gray-900">Alert received!</p>
                    <p className="mt-1 text-sm text-gray-500">Incident created successfully.</p>
                  </>
                )}

                {testStatus === 'error' && (
                  <>
                    <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-red-50">
                      <AlertTriangle className="h-10 w-10 text-red-500" />
                    </div>
                    <p className="text-lg font-semibold text-gray-900">Something went wrong</p>
                    <p className="mt-1 text-sm text-gray-500">The test alert could not be delivered.</p>
                    <button
                      onClick={() => {
                        setTestStatus('idle');
                      }}
                      className="mt-4 text-sm text-blue-600 hover:text-blue-700 font-medium"
                    >
                      Try again
                    </button>
                  </>
                )}
              </div>

              <div className="mt-4 flex justify-between">
                <button
                  onClick={() => goTo(2)}
                  className="inline-flex items-center gap-1 rounded-lg border border-gray-300 px-4 py-2.5 text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors"
                >
                  <ArrowLeft className="h-4 w-4" /> Back
                </button>
                <div className="flex gap-2">
                  {testStatus !== 'success' && (
                    <button
                      onClick={() => goTo(4)}
                      className="inline-flex items-center gap-1 rounded-lg border border-gray-300 px-4 py-2.5 text-sm font-medium text-gray-500 hover:bg-gray-50 transition-colors"
                    >
                      <SkipForward className="h-4 w-4" /> Skip
                    </button>
                  )}
                  <button
                    onClick={() => goTo(4)}
                    disabled={testStatus !== 'success'}
                    className={`inline-flex items-center gap-2 rounded-lg px-5 py-2.5 text-sm font-semibold text-white transition-colors ${
                      testStatus === 'success'
                        ? 'bg-green-600 hover:bg-green-700'
                        : 'bg-gray-300 cursor-not-allowed'
                    }`}
                  >
                    Next <ArrowRight className="h-4 w-4" />
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* Step 4: Done */}
          {step === 4 && (
            <div className="text-center py-4">
              <div className="mx-auto mb-5 flex h-20 w-20 items-center justify-center rounded-2xl bg-gradient-to-br from-green-400 to-emerald-600 shadow-lg shadow-green-200">
                <CheckCircle className="h-10 w-10 text-white" />
              </div>
              <h1 className="text-3xl font-bold text-gray-900">Your RootCauseway instance is ready!</h1>
              <p className="mx-auto mt-4 max-w-md text-gray-500">
                1 service configured, 1 webhook connected.
                {testStatus === 'success'
                  ? ' Your test alert was received and an incident was created.'
                  : ' Start sending alerts to create incidents.'}
              </p>

              <div className="mt-4 flex justify-center gap-6 text-sm">
                <div className="flex items-center gap-1.5 text-green-600">
                  <CheckCircle className="h-4 w-4" />
                  <span className="font-medium">{softwareName}</span>
                </div>
                <div className="flex items-center gap-1.5 text-green-600">
                  <CheckCircle className="h-4 w-4" />
                  <span className="font-medium">{selectedSourceInfo?.name ?? 'Webhook'}</span>
                </div>
              </div>

              <button
                onClick={() => navigate('/incidents')}
                className="mt-8 inline-flex items-center gap-2 rounded-lg bg-blue-600 px-8 py-3 text-sm font-semibold text-white shadow-sm hover:bg-blue-700 hover:shadow-md transition-all"
              >
                Go to Dashboard <ArrowRight className="h-4 w-4" />
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
