import { FileText, BarChart3, Activity, Camera, Bot, PenTool, ExternalLink } from 'lucide-react';
import type { IncidentEvidence, EvidenceType } from '@/types/api';

const iconMap: Record<EvidenceType, typeof FileText> = {
  log: FileText,
  metric: BarChart3,
  trace: Activity,
  snapshot: Camera,
  agent_output: Bot,
  manual: PenTool,
};

const colorMap: Record<EvidenceType, string> = {
  log: 'text-green-600 bg-green-50',
  metric: 'text-blue-600 bg-blue-50',
  trace: 'text-purple-600 bg-purple-50',
  snapshot: 'text-orange-600 bg-orange-50',
  agent_output: 'text-cyan-600 bg-cyan-50',
  manual: 'text-gray-600 bg-gray-50',
};

export function EvidencePanel({ evidence }: { evidence: IncidentEvidence[] }) {
  if (evidence.length === 0) {
    return <p className="text-sm text-gray-500">No evidence collected yet.</p>;
  }

  return (
    <div className="space-y-3" data-testid="evidence-panel">
      {evidence.map((item) => {
        const Icon = iconMap[item.type] ?? FileText;
        const color = colorMap[item.type] ?? 'text-gray-600 bg-gray-50';

        return (
          <div key={item.id} className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
            <div className="flex items-start gap-3">
              <span className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-lg ${color}`}>
                <Icon className="h-4 w-4" />
              </span>
              <div className="min-w-0 flex-1">
                <div className="flex items-center justify-between">
                  <h4 className="text-sm font-medium text-gray-900">{item.title}</h4>
                  <span className="rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-600">{item.type}</span>
                </div>
                {item.source && (
                  <p className="mt-0.5 text-xs text-gray-500">Source: {item.source}</p>
                )}
                {item.source === 'mlflow' && typeof item.content?.url === 'string' ? (
                  <a
                    href={item.content.url}
                    target="_blank"
                    rel="noreferrer"
                    className="mt-2 inline-flex items-center gap-1 text-xs font-medium text-purple-700 hover:underline"
                  >
                    View trace in MLflow <ExternalLink className="h-3 w-3" />
                  </a>
                ) : (
                  <pre className="mt-2 max-h-40 overflow-auto rounded bg-gray-50 p-2 text-xs text-gray-700">
                    {JSON.stringify(item.content, null, 2)}
                  </pre>
                )}
                <p className="mt-1 text-xs text-gray-400">
                  {new Date(item.collected_at).toLocaleString()}
                </p>
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}
