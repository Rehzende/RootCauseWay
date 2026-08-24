import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { Search, CheckCircle, Download, Star, Check } from 'lucide-react';
import { useToast } from '@/components/Toast';
import { listMarketplaceAgents, listInstalledAgents, installAgent } from '@/services/api';
import type { MarketplaceAgent } from '@/types/api';

const categories: Array<{ value: string; label: string }> = [
  { value: 'all', label: 'All' },
  { value: 'triage', label: 'Triage' },
  { value: 'evidence', label: 'Evidence' },
  { value: 'rca', label: 'RCA' },
  { value: 'postmortem', label: 'Postmortem' },
  { value: 'security', label: 'Security' },
  { value: 'infrastructure', label: 'Infrastructure' },
  { value: 'database', label: 'Database' },
  { value: 'cloud', label: 'Cloud' },
];

const categoryBorderColor: Record<string, string> = {
  triage: 'border-t-blue-500',
  evidence: 'border-t-teal-500',
  rca: 'border-t-red-500',
  postmortem: 'border-t-purple-500',
  security: 'border-t-red-500',
  infrastructure: 'border-t-gray-500',
  database: 'border-t-purple-500',
  cloud: 'border-t-cyan-500',
  custom: 'border-t-gray-400',
};

const categoryBgColor: Record<string, string> = {
  triage: 'bg-blue-100 text-blue-800',
  evidence: 'bg-teal-100 text-teal-800',
  rca: 'bg-red-100 text-red-800',
  postmortem: 'bg-purple-100 text-purple-800',
  security: 'bg-red-100 text-red-800',
  infrastructure: 'bg-gray-100 text-gray-800',
  database: 'bg-purple-100 text-purple-800',
  cloud: 'bg-cyan-100 text-cyan-800',
  custom: 'bg-gray-100 text-gray-800',
};

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
          className={`h-3.5 w-3.5 ${i <= Math.round(rating) ? 'fill-amber-400 text-amber-400' : 'text-gray-300'}`}
        />
      ))}
    </div>
  );
}

function AgentCard({
  agent,
  isInstalled,
  onInstall,
  installing,
  onClick,
}: {
  agent: MarketplaceAgent;
  isInstalled: boolean;
  onInstall: () => void;
  installing: boolean;
  onClick: () => void;
}) {
  return (
    <div
      onClick={onClick}
      className={`cursor-pointer rounded-lg border border-t-2 bg-white shadow-sm transition hover:shadow-md ${categoryBorderColor[agent.category] ?? 'border-t-gray-400'}`}
    >
      <div className="p-5">
        <div className="flex items-start gap-3">
          <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-white font-bold text-lg ${categoryIconBg[agent.category] ?? 'bg-gray-400'}`}>
            {agent.name.charAt(0).toUpperCase()}
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-1.5">
              <h3 className="truncate font-semibold text-gray-900">{agent.name}</h3>
              {agent.verified && <CheckCircle className="h-4 w-4 shrink-0 text-blue-500" />}
            </div>
            <p className="text-xs text-gray-500">{agent.author}</p>
          </div>
        </div>

        {agent.description && (
          <p className="mt-3 text-sm text-gray-600 line-clamp-2">{agent.description}</p>
        )}

        <div className="mt-3 flex flex-wrap items-center gap-1.5">
          <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${categoryBgColor[agent.category] ?? 'bg-gray-100 text-gray-800'}`}>
            {agent.category}
          </span>
          {agent.skills.slice(0, 3).map((s) => (
            <span key={s.id} className="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600">
              {s.name}
            </span>
          ))}
          {agent.skills.length > 3 && (
            <span className="text-xs text-gray-400">+{agent.skills.length - 3} more</span>
          )}
        </div>

        <div className="mt-3 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <RatingStars rating={agent.rating} />
            <span className="flex items-center gap-1 text-xs text-gray-400">
              <Download className="h-3 w-3" />
              {agent.downloads.toLocaleString()}
            </span>
          </div>
          <span className="text-xs text-gray-400">v{agent.version}</span>
        </div>
      </div>

      <div className="border-t border-gray-100 px-5 py-3" onClick={(e) => e.stopPropagation()}>
        {isInstalled ? (
          <span className="inline-flex items-center gap-1.5 rounded-md bg-green-50 px-3 py-1.5 text-xs font-medium text-green-700">
            <Check className="h-3.5 w-3.5" /> Installed
          </span>
        ) : (
          <button
            onClick={onInstall}
            disabled={installing}
            className="inline-flex items-center gap-1.5 rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-700 disabled:opacity-50"
          >
            {installing ? 'Installing...' : 'Install'}
          </button>
        )}
      </div>
    </div>
  );
}

export function MarketplacePage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [search, setSearch] = useState('');
  const [category, setCategory] = useState('all');

  const { data: agents, isLoading } = useQuery({
    queryKey: ['marketplace-agents', category, search],
    queryFn: () =>
      listMarketplaceAgents({
        category: category === 'all' ? undefined : category,
        search: search || undefined,
      }),
  });

  const { data: installed } = useQuery({
    queryKey: ['installed-agents'],
    queryFn: listInstalledAgents,
  });

  const installMut = useMutation({
    mutationFn: (slug: string) => installAgent(slug),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['installed-agents'] }); addToast({ type: 'success', title: 'Agent installed successfully' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to install agent', message: err?.response?.data?.error || err.message }); },
  });

  const installedMarketplaceIds = new Set((installed ?? []).map((i) => i.marketplace_agent_id));

  return (
    <div className="p-8">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Agent Marketplace</h1>
        <p className="mt-1 text-sm text-gray-500">Discover and install agents to enhance your incident analysis</p>
      </div>

      {/* Search */}
      <div className="relative mb-6">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search agents..."
          className="w-full rounded-lg border border-gray-300 py-2.5 pl-10 pr-4 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
        />
      </div>

      {/* Category tabs */}
      <div className="mb-6 flex flex-wrap gap-2">
        {categories.map((cat) => (
          <button
            key={cat.value}
            onClick={() => setCategory(cat.value)}
            className={`rounded-full px-4 py-1.5 text-sm font-medium transition ${
              category === cat.value
                ? 'bg-gray-900 text-white'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
            }`}
          >
            {cat.label}
          </button>
        ))}
      </div>

      {/* Grid */}
      {isLoading ? (
        <p className="text-sm text-gray-500">Loading...</p>
      ) : (agents ?? []).length === 0 ? (
        <div className="rounded-lg border border-dashed border-gray-300 p-12 text-center">
          <p className="text-sm text-gray-500">No agents found</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {(agents ?? []).map((agent) => (
            <AgentCard
              key={agent.id}
              agent={agent}
              isInstalled={installedMarketplaceIds.has(agent.id)}
              onInstall={() => installMut.mutate(agent.slug)}
              installing={installMut.isPending && installMut.variables === agent.slug}
              onClick={() => navigate(`/marketplace/${agent.slug}`)}
            />
          ))}
        </div>
      )}
    </div>
  );
}
