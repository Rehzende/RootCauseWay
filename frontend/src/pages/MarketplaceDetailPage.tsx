import { useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  ArrowLeft, CheckCircle, Download, Star, Check, Trash2,
} from 'lucide-react';
import {
  getMarketplaceAgent, listInstalledAgents, installAgent, uninstallAgent,
} from '@/services/api';

type Tab = 'overview' | 'readme' | 'configuration' | 'skills';

const categoryIconBg: Record<string, string> = {
  triage: 'bg-blue-500',
  evidence: 'bg-teal-500',
  rca: 'bg-red-500',
  postmortem: 'bg-purple-500',
  security: 'bg-red-600',
  infrastructure: 'bg-gray-500',
  database: 'bg-purple-600',
  cloud: 'bg-cyan-500',
  custom: 'bg-gray-400',
};

function RatingStars({ rating }: { rating: number }) {
  return (
    <div className="flex items-center gap-0.5">
      {[1, 2, 3, 4, 5].map((i) => (
        <Star
          key={i}
          className={`h-4 w-4 ${i <= Math.round(rating) ? 'fill-amber-400 text-amber-400' : 'text-gray-300'}`}
        />
      ))}
    </div>
  );
}

function ConfigSchemaForm({ schema }: { schema: Record<string, unknown> }) {
  const properties = (schema.properties ?? {}) as Record<string, Record<string, unknown>>;
  const entries = Object.entries(properties);
  if (entries.length === 0) {
    return <p className="text-sm text-gray-500">No configuration required</p>;
  }
  return (
    <div className="space-y-4">
      {entries.map(([key, prop]) => (
        <div key={key}>
          <label className="mb-1 block text-sm font-medium text-gray-700">
            {key}
            {prop.type != null && <span className="ml-1 text-xs text-gray-400">({`${prop.type}`})</span>}
          </label>
          {prop.description != null && <p className="mb-1 text-xs text-gray-500">{`${prop.description}`}</p>}
          {prop.type === 'boolean' ? (
            <input type="checkbox" className="h-4 w-4 rounded border-gray-300" />
          ) : prop.type === 'number' || prop.type === 'integer' ? (
            <input
              type="number"
              placeholder={prop.default != null ? String(prop.default) : ''}
              className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none"
            />
          ) : (
            <input
              type="text"
              placeholder={prop.default != null ? String(prop.default) : ''}
              className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none"
            />
          )}
        </div>
      ))}
    </div>
  );
}

export function MarketplaceDetailPage() {
  const { slug } = useParams<{ slug: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [tab, setTab] = useState<Tab>('overview');

  const { data: agent, isLoading } = useQuery({
    queryKey: ['marketplace-agent', slug],
    queryFn: () => getMarketplaceAgent(slug!),
    enabled: !!slug,
  });

  const { data: installed } = useQuery({
    queryKey: ['installed-agents'],
    queryFn: listInstalledAgents,
  });

  const installMut = useMutation({
    mutationFn: () => installAgent(slug!),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['installed-agents'] }),
  });

  const uninstallMut = useMutation({
    mutationFn: (id: string) => uninstallAgent(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['installed-agents'] }),
  });

  if (isLoading || !agent) {
    return (
      <div className="p-8">
        <p className="text-sm text-gray-500">Loading...</p>
      </div>
    );
  }

  const installedEntry = (installed ?? []).find((i) => i.marketplace_agent_id === agent.id);
  const isInstalled = !!installedEntry;

  const tabs: Array<{ key: Tab; label: string }> = [
    { key: 'overview', label: 'Overview' },
    { key: 'readme', label: 'README' },
    { key: 'configuration', label: 'Configuration' },
    { key: 'skills', label: 'Skills' },
  ];

  return (
    <div className="p-8">
      <button
        onClick={() => navigate('/marketplace')}
        className="mb-6 inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-700"
      >
        <ArrowLeft className="h-4 w-4" /> Back to Marketplace
      </button>

      {/* Header */}
      <div className="flex items-start justify-between">
        <div className="flex items-start gap-4">
          <div className={`flex h-14 w-14 shrink-0 items-center justify-center rounded-xl text-white font-bold text-2xl ${categoryIconBg[agent.category] ?? 'bg-gray-400'}`}>
            {agent.name.charAt(0).toUpperCase()}
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-2xl font-bold text-gray-900">{agent.name}</h1>
              {agent.verified && <CheckCircle className="h-5 w-5 text-blue-500" />}
              <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
                v{agent.version}
              </span>
            </div>
            <p className="mt-1 text-sm text-gray-500">
              by{' '}
              {agent.author_url ? (
                <a href={agent.author_url} className="text-blue-600 hover:underline" target="_blank" rel="noreferrer">
                  {agent.author}
                </a>
              ) : (
                agent.author
              )}
            </p>
            <div className="mt-2 flex items-center gap-4">
              <RatingStars rating={agent.rating} />
              <span className="flex items-center gap-1 text-sm text-gray-500">
                <Download className="h-4 w-4" />
                {agent.downloads.toLocaleString()} downloads
              </span>
            </div>
          </div>
        </div>

        <div>
          {isInstalled ? (
            <div className="flex items-center gap-2">
              <span className="inline-flex items-center gap-1.5 rounded-md bg-green-50 px-4 py-2 text-sm font-medium text-green-700">
                <Check className="h-4 w-4" /> Installed
              </span>
              <button
                onClick={() => uninstallMut.mutate(installedEntry!.id)}
                disabled={uninstallMut.isPending}
                className="inline-flex items-center gap-1.5 rounded-md border border-red-200 px-3 py-2 text-sm font-medium text-red-600 hover:bg-red-50 disabled:opacity-50"
              >
                <Trash2 className="h-4 w-4" />
              </button>
            </div>
          ) : (
            <button
              onClick={() => installMut.mutate()}
              disabled={installMut.isPending}
              className="rounded-md bg-blue-600 px-5 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
            >
              {installMut.isPending ? 'Installing...' : 'Install Agent'}
            </button>
          )}
        </div>
      </div>

      {/* Tabs */}
      <div className="mt-8 border-b border-gray-200">
        <div className="flex gap-6">
          {tabs.map((t) => (
            <button
              key={t.key}
              onClick={() => setTab(t.key)}
              className={`border-b-2 pb-3 text-sm font-medium transition ${
                tab === t.key
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700'
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>

      {/* Tab content */}
      <div className="mt-6">
        {tab === 'overview' && (
          <div className="space-y-6">
            {agent.long_description && (
              <div>
                <h3 className="mb-2 text-sm font-semibold text-gray-900">Description</h3>
                <p className="text-sm text-gray-700 leading-relaxed">{agent.long_description}</p>
              </div>
            )}
            {agent.required_credentials.length > 0 && (
              <div>
                <h3 className="mb-2 text-sm font-semibold text-gray-900">Required Credentials</h3>
                <div className="flex flex-wrap gap-2">
                  {agent.required_credentials.map((c) => (
                    <span key={c} className="rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs font-medium text-amber-800">
                      {c}
                    </span>
                  ))}
                </div>
              </div>
            )}
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
              <div className="rounded-lg border border-gray-200 p-4">
                <p className="text-xs text-gray-500">Downloads</p>
                <p className="mt-1 text-lg font-semibold text-gray-900">{agent.downloads.toLocaleString()}</p>
              </div>
              <div className="rounded-lg border border-gray-200 p-4">
                <p className="text-xs text-gray-500">Rating</p>
                <p className="mt-1 text-lg font-semibold text-gray-900">{agent.rating.toFixed(1)}</p>
              </div>
              <div className="rounded-lg border border-gray-200 p-4">
                <p className="text-xs text-gray-500">Version</p>
                <p className="mt-1 text-lg font-semibold text-gray-900">{agent.version}</p>
              </div>
              <div className="rounded-lg border border-gray-200 p-4">
                <p className="text-xs text-gray-500">Category</p>
                <p className="mt-1 text-lg font-semibold text-gray-900 capitalize">{agent.category}</p>
              </div>
            </div>
          </div>
        )}

        {tab === 'readme' && (
          <div className="rounded-lg border border-gray-200 bg-white p-6">
            {agent.readme ? (
              <pre className="whitespace-pre-wrap text-sm text-gray-700 leading-relaxed font-sans">
                {agent.readme}
              </pre>
            ) : (
              <p className="text-sm text-gray-500">No README available</p>
            )}
          </div>
        )}

        {tab === 'configuration' && (
          <div className="rounded-lg border border-gray-200 bg-white p-6">
            <h3 className="mb-4 text-sm font-semibold text-gray-900">Configuration Schema</h3>
            <ConfigSchemaForm schema={agent.config_schema} />
          </div>
        )}

        {tab === 'skills' && (
          <div className="space-y-3">
            {agent.skills.length === 0 ? (
              <p className="text-sm text-gray-500">No skills defined</p>
            ) : (
              agent.skills.map((s) => (
                <div key={s.id} className="rounded-lg border border-gray-200 bg-white px-5 py-4">
                  <p className="font-medium text-gray-900">{s.name}</p>
                  {s.description && <p className="mt-1 text-sm text-gray-600">{s.description}</p>}
                </div>
              ))
            )}
          </div>
        )}
      </div>
    </div>
  );
}
