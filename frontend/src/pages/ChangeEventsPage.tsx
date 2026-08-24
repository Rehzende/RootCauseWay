import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { listChangeEvents } from '@/services/api';
import type { ChangeEvent } from '@/types/api';

const TYPE_COLORS: Record<string, string> = {
  deploy: 'bg-green-100 text-green-700 border-green-300',
  config_change: 'bg-yellow-100 text-yellow-700 border-yellow-300',
  infra_change: 'bg-purple-100 text-purple-700 border-purple-300',
  rollback: 'bg-red-100 text-red-700 border-red-300',
};

export function ChangeEventsPage() {
  const [softwareId, setSoftwareId] = useState('');

  const { data, isLoading } = useQuery({
    queryKey: ['change-events', softwareId],
    queryFn: () => listChangeEvents(softwareId ? { software_id: softwareId } : undefined),
  });

  const events: ChangeEvent[] = data?.data ?? [];

  return (
    <div className="p-8 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Change Events</h1>
          <p className="text-gray-500 text-sm mt-1">Deploys, config changes, and infra modifications</p>
        </div>
      </div>

      <div className="flex gap-3">
        <input
          value={softwareId}
          onChange={e => setSoftwareId(e.target.value)}
          placeholder="Filter by software ID..."
          className="w-64 border border-gray-300 rounded-lg px-4 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
      </div>

      {isLoading ? (
        <div className="flex items-center justify-center h-40">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
        </div>
      ) : events.length === 0 ? (
        <div className="text-center py-16 text-gray-400">
          <p className="text-lg font-medium text-gray-600">No change events recorded</p>
          <p className="text-sm mt-1">Register deploys and config changes to correlate with incidents.</p>
        </div>
      ) : (
        <div className="space-y-2">
          {events.map(ev => (
            <div key={ev.id} className="bg-white border border-gray-200 rounded-lg p-4 flex items-start gap-4 hover:shadow-sm transition-shadow">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className={`px-2 py-0.5 text-xs rounded border font-medium ${TYPE_COLORS[ev.change_type] ?? 'bg-gray-100 text-gray-600 border-gray-300'}`}>
                    {ev.change_type.replace(/_/g, ' ')}
                  </span>
                  <span className="text-gray-900 font-medium truncate">{ev.title}</span>
                </div>
                <div className="flex items-center gap-3 mt-1.5 flex-wrap">
                  {ev.author && (
                    <span className="text-gray-500 text-xs">by {ev.author}</span>
                  )}
                  {ev.source && (
                    <span className="text-gray-400 text-xs">{ev.source}</span>
                  )}
                  {ev.commit_sha && (
                    <span className="text-gray-400 text-xs font-mono">{ev.commit_sha.slice(0, 8)}</span>
                  )}
                  {ev.environment && (
                    <span className="text-gray-500 text-xs bg-gray-100 px-1.5 py-0.5 rounded">{ev.environment}</span>
                  )}
                  {ev.source_url && (
                    <a
                      href={ev.source_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-blue-500 text-xs hover:underline"
                    >
                      View
                    </a>
                  )}
                </div>
                {ev.description && (
                  <p className="text-gray-500 text-sm mt-1">{ev.description}</p>
                )}
              </div>
              <span className="text-gray-400 text-xs shrink-0 whitespace-nowrap">
                {new Date(ev.occurred_at).toLocaleString()}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
