import { useQuery } from '@tanstack/react-query';
import { Brain, Bot, Clock } from 'lucide-react';
import { listOrchestratorDecisions } from '@/services/api';
import type { OrchestratorDecision } from '@/types/api';

function ConfidenceBar({ confidence }: { confidence: number }) {
  const pct = Math.round(confidence * 100);
  const color = pct >= 80 ? 'bg-green-500' : pct >= 50 ? 'bg-yellow-500' : 'bg-red-500';
  return (
    <div className="flex items-center gap-2">
      <div className="h-2 w-24 rounded-full bg-gray-200">
        <div className={`h-2 rounded-full ${color}`} style={{ width: `${pct}%` }} />
      </div>
      <span className="text-xs font-medium text-gray-600">{pct}%</span>
    </div>
  );
}

function DecisionCard({ decision }: { decision: OrchestratorDecision }) {
  const typeColors: Record<string, string> = {
    agent_selection: 'bg-blue-100 text-blue-800',
    escalation: 'bg-red-100 text-red-800',
    routing: 'bg-purple-100 text-purple-800',
  };
  const badge = typeColors[decision.decision_type] ?? 'bg-gray-100 text-gray-800';

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4">
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-2">
          <Brain className="h-4 w-4 text-purple-500" />
          <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${badge}`}>
            {decision.decision_type.replace(/_/g, ' ')}
          </span>
        </div>
        <span className="flex items-center gap-1 text-xs text-gray-400">
          <Clock className="h-3 w-3" />
          {new Date(decision.created_at).toLocaleString()}
        </span>
      </div>

      <p className="mt-3 text-sm text-gray-700">{decision.reasoning}</p>

      <div className="mt-3 flex flex-wrap gap-1.5">
        {decision.selected_agents.map((agent) => (
          <span key={agent} className="inline-flex items-center gap-1 rounded-full bg-indigo-50 px-2.5 py-0.5 text-xs font-medium text-indigo-700">
            <Bot className="h-3 w-3" />
            {agent}
          </span>
        ))}
      </div>

      <div className="mt-3">
        <ConfidenceBar confidence={decision.confidence} />
      </div>
    </div>
  );
}

export function OrchestratorDecisions({ incidentId }: { incidentId: string }) {
  const { data: decisions, isLoading } = useQuery({
    queryKey: ['orchestrator-decisions', incidentId],
    queryFn: () => listOrchestratorDecisions(incidentId),
  });

  if (isLoading) {
    return <p className="text-sm text-gray-500">Loading decisions...</p>;
  }

  if (!decisions?.length) {
    return (
      <div className="rounded-lg border border-dashed border-gray-300 p-6 text-center">
        <Brain className="mx-auto h-8 w-8 text-gray-300" />
        <p className="mt-2 text-sm text-gray-500">No orchestrator decisions yet</p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {decisions.map((d) => (
        <DecisionCard key={d.id} decision={d} />
      ))}
    </div>
  );
}
