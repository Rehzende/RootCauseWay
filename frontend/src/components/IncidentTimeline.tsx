import { AlertTriangle, Search, CheckCircle, FileText, Lightbulb, ArrowRightLeft, MessageSquare, Bot, XCircle, Link2, Video } from 'lucide-react';
import type { IncidentEvent, IncidentEventType } from '@/types/api';

const iconMap: Record<IncidentEventType, typeof AlertTriangle> = {
  alert_received: AlertTriangle,
  triage_started: Search,
  triage_completed: CheckCircle,
  evidence_collected: FileText,
  hypothesis_generated: Lightbulb,
  status_changed: ArrowRightLeft,
  comment: MessageSquare,
  agent_action: Bot,
  rci_started: Search,
  rci_completed: CheckCircle,
  rca_started: Search,
  rca_completed: CheckCircle,
  postmortem_started: FileText,
  postmortem_completed: CheckCircle,
  agent_run_started: Bot,
  agent_run_completed: CheckCircle,
  agent_run_failed: XCircle,
  correlated_alert: Link2,
  war_room_created: Video,
};

const colorMap: Record<IncidentEventType, string> = {
  alert_received: 'text-red-500 bg-red-50',
  triage_started: 'text-blue-500 bg-blue-50',
  triage_completed: 'text-green-500 bg-green-50',
  evidence_collected: 'text-indigo-500 bg-indigo-50',
  hypothesis_generated: 'text-yellow-500 bg-yellow-50',
  status_changed: 'text-purple-500 bg-purple-50',
  comment: 'text-gray-500 bg-gray-50',
  agent_action: 'text-cyan-500 bg-cyan-50',
  rci_started: 'text-blue-500 bg-blue-50',
  rci_completed: 'text-green-500 bg-green-50',
  rca_started: 'text-blue-500 bg-blue-50',
  rca_completed: 'text-green-500 bg-green-50',
  postmortem_started: 'text-blue-500 bg-blue-50',
  postmortem_completed: 'text-green-500 bg-green-50',
  agent_run_started: 'text-sky-500 bg-sky-50',
  agent_run_completed: 'text-emerald-500 bg-emerald-50',
  agent_run_failed: 'text-red-600 bg-red-50',
  correlated_alert: 'text-orange-500 bg-orange-50',
  war_room_created: 'text-pink-500 bg-pink-50',
};

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString();
}

function eventLabel(type: IncidentEventType): string {
  return type.split('_').map((w) => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
}

export function IncidentTimeline({ events }: { events: IncidentEvent[] }) {
  if (events.length === 0) {
    return <p className="text-sm text-gray-500">No events yet.</p>;
  }

  return (
    <div className="flow-root" data-testid="incident-timeline">
      <ul className="-mb-8">
        {events.map((event, idx) => {
          const Icon = iconMap[event.type] ?? MessageSquare;
          const color = colorMap[event.type] ?? 'text-gray-500 bg-gray-50';
          const isLast = idx === events.length - 1;

          return (
            <li key={event.id} data-testid={`timeline-event-${event.type}`}>
              <div className="relative pb-8">
                {!isLast && (
                  <span className="absolute left-4 top-4 -ml-px h-full w-0.5 bg-gray-200" aria-hidden="true" />
                )}
                <div className="relative flex space-x-3">
                  <div>
                    <span className={`flex h-8 w-8 items-center justify-center rounded-full ring-4 ring-white ${color}`}>
                      <Icon className="h-4 w-4" />
                    </span>
                  </div>
                  <div className="flex min-w-0 flex-1 justify-between space-x-4 pt-1">
                    <div>
                      <p className="text-sm font-medium text-gray-900">{eventLabel(event.type)}</p>
                      <p className="text-xs text-gray-500">by {event.actor}</p>
                      {event.data && Object.keys(event.data).length > 0 && (
                        <p className="mt-1 text-xs text-gray-600">
                          {JSON.stringify(event.data)}
                        </p>
                      )}
                    </div>
                    <div className="whitespace-nowrap text-right text-xs text-gray-500">
                      {formatTime(event.created_at)}
                    </div>
                  </div>
                </div>
              </div>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
