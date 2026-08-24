import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { searchKnowledgeBase } from '@/services/api';
import type { KnowledgeBaseEntry } from '@/types/api';

export function KnowledgeBasePage() {
  const [query, setQuery] = useState('');
  const [category, setCategory] = useState('');

  const { data, isLoading } = useQuery({
    queryKey: ['knowledge-base', query, category],
    queryFn: () => searchKnowledgeBase({ query: query || undefined, category: category || undefined }),
  });

  const items: KnowledgeBaseEntry[] = data ?? [];

  return (
    <div className="p-8 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Knowledge Base</h1>
          <p className="text-gray-500 text-sm mt-1">Lessons learned from past incidents</p>
        </div>
      </div>

      <div className="flex gap-3">
        <input
          value={query}
          onChange={e => setQuery(e.target.value)}
          placeholder="Search knowledge base..."
          className="flex-1 border border-gray-300 rounded-lg px-4 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
        <input
          value={category}
          onChange={e => setCategory(e.target.value)}
          placeholder="Category filter..."
          className="w-48 border border-gray-300 rounded-lg px-4 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
      </div>

      {isLoading ? (
        <div className="flex items-center justify-center h-40">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
        </div>
      ) : items.length === 0 ? (
        <div className="text-center py-16 text-gray-400">
          <p className="text-lg font-medium text-gray-600">No knowledge base entries found</p>
          <p className="text-sm mt-1">Entries are auto-created when incidents resolve.</p>
        </div>
      ) : (
        <div className="space-y-4">
          {items.map(kb => (
            <div key={kb.id} className="bg-white border border-gray-200 rounded-lg p-5 hover:shadow-sm transition-shadow">
              <div className="flex items-start gap-4">
                <div className="flex-1 space-y-2">
                  <div className="flex items-center gap-2 flex-wrap">
                    {kb.human_validated && (
                      <span className="px-2 py-0.5 bg-green-100 text-green-700 text-xs rounded border border-green-300 font-medium">
                        Human Validated
                      </span>
                    )}
                    {kb.category && (
                      <span className="px-2 py-0.5 bg-gray-100 text-gray-600 text-xs rounded">{kb.category}</span>
                    )}
                    {kb.tags?.map(tag => (
                      <span key={tag} className="px-2 py-0.5 bg-blue-50 text-blue-600 text-xs rounded">{tag}</span>
                    ))}
                    <span className="text-gray-400 text-xs ml-auto">
                      Referenced {kb.times_referenced}x &middot; Confidence {Math.round(kb.confidence * 100)}%
                    </span>
                  </div>

                  <div>
                    <p className="text-gray-400 text-xs uppercase tracking-wide mb-0.5">Root Cause</p>
                    <p className="text-gray-900 text-sm">{kb.root_cause_summary}</p>
                  </div>

                  {kb.resolution_summary && (
                    <div>
                      <p className="text-gray-400 text-xs uppercase tracking-wide mb-0.5">Resolution</p>
                      <p className="text-gray-700 text-sm">{kb.resolution_summary}</p>
                    </div>
                  )}

                  {kb.lessons_learned && kb.lessons_learned.length > 0 && (
                    <div>
                      <p className="text-gray-400 text-xs uppercase tracking-wide mb-0.5">Lessons Learned</p>
                      <ul className="list-disc list-inside space-y-0.5">
                        {kb.lessons_learned.map((lesson, i) => (
                          <li key={i} className="text-gray-600 text-sm">{lesson}</li>
                        ))}
                      </ul>
                    </div>
                  )}

                  <p className="text-gray-400 text-xs">
                    Added {new Date(kb.created_at).toLocaleDateString()}
                    {kb.incident_id && (
                      <span> &middot; from incident</span>
                    )}
                  </p>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
