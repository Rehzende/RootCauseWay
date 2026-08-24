import { useState } from 'react';
import { Video, ExternalLink, Square, Users, ListChecks, Copy, Check, Clock, Radio } from 'lucide-react';
import { useWarRoom, useStartWarRoom, useEndWarRoom } from '@/hooks/useWarRoom';
import { Skeleton } from '@/components/Skeleton';
import { useToast } from '@/components/Toast';
import type { WarRoomStatus } from '@/types/api';

interface WarRoomPanelProps {
  incidentId: string;
}

const statusStyle: Record<WarRoomStatus, string> = {
  scheduled: 'bg-gray-100 text-gray-700 border-gray-200',
  active: 'bg-green-100 text-green-700 border-green-200',
  ended: 'bg-amber-100 text-amber-700 border-amber-200',
  summarized: 'bg-purple-100 text-purple-700 border-purple-200',
};

const statusDotStyle: Record<WarRoomStatus, string> = {
  scheduled: 'bg-gray-400',
  active: 'bg-green-500',
  ended: 'bg-amber-500',
  summarized: 'bg-purple-500',
};

function elapsedSince(iso: string): string {
  const ms = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(ms / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m`;
  const hours = Math.floor(mins / 60);
  return `${hours}h ${mins % 60}m`;
}

export function WarRoomPanel({ incidentId }: WarRoomPanelProps) {
  const { data: meeting, isLoading } = useWarRoom(incidentId);
  const startMut = useStartWarRoom(incidentId);
  const endMut = useEndWarRoom(incidentId);
  const { addToast } = useToast();
  const [copied, setCopied] = useState(false);

  const copyJoinLink = async () => {
    if (!meeting?.join_url) return;
    try {
      await navigator.clipboard.writeText(meeting.join_url);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      addToast({ type: 'error', title: 'Failed to copy link' });
    }
  };

  if (isLoading) {
    return (
      <div className="space-y-3">
        <Skeleton className="h-4 w-32" />
        <Skeleton className="h-16 w-full" />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="flex items-center gap-2 text-sm font-semibold text-gray-900">
          <Video className="h-4 w-4 text-gray-400" /> War Room
        </h3>
        {meeting && (
          <span className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-medium ${statusStyle[meeting.status]}`}>
            <span className={`h-1.5 w-1.5 rounded-full ${statusDotStyle[meeting.status]} ${meeting.status === 'active' ? 'animate-pulse' : ''}`} />
            {meeting.status}
          </span>
        )}
      </div>

      {!meeting ? (
        <div className="flex flex-col items-start gap-3 rounded-lg border border-dashed border-gray-200 bg-gray-50 px-4 py-5">
          <p className="text-sm text-gray-500">
            No war room meeting has been started for this incident yet.
          </p>
          <button
            onClick={() => startMut.mutate()}
            disabled={startMut.isPending}
            className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-blue-700 disabled:opacity-50"
          >
            <Video className="h-4 w-4" />
            {startMut.isPending ? 'Starting...' : 'Start War Room'}
          </button>
        </div>
      ) : (
        <div className="space-y-4">
          {meeting.join_url && meeting.status !== 'summarized' && (
            <div className="rounded-lg border border-gray-200 bg-white shadow-sm">
              <div className="flex flex-wrap items-center gap-3 p-4">
                <a
                  href={meeting.join_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-blue-700"
                >
                  <ExternalLink className="h-4 w-4" /> Join Teams Meeting
                </a>
                <button
                  onClick={copyJoinLink}
                  className="inline-flex items-center gap-1.5 rounded-md border border-gray-300 bg-white px-3 py-2 text-sm font-medium text-gray-600 hover:bg-gray-50"
                  title="Copy join link"
                >
                  {copied ? <Check className="h-3.5 w-3.5 text-green-600" /> : <Copy className="h-3.5 w-3.5" />}
                  {copied ? 'Copied' : 'Copy link'}
                </button>

                {(meeting.status === 'scheduled' || meeting.status === 'active') && (
                  <button
                    onClick={() => endMut.mutate(meeting.id)}
                    disabled={endMut.isPending}
                    className="ml-auto inline-flex items-center gap-2 rounded-md border border-red-200 bg-red-50 px-4 py-2 text-sm font-medium text-red-700 hover:bg-red-100 disabled:opacity-50"
                  >
                    <Square className="h-3.5 w-3.5" />
                    {endMut.isPending ? 'Ending...' : 'End War Room'}
                  </button>
                )}
              </div>

              {meeting.started_at && (
                <div className="flex items-center gap-1.5 border-t border-gray-100 px-4 py-2 text-xs text-gray-500">
                  <Clock className="h-3.5 w-3.5" />
                  {meeting.status === 'active' || meeting.status === 'scheduled' ? (
                    <span>Live for {elapsedSince(meeting.started_at)}</span>
                  ) : (
                    <span>Started {new Date(meeting.started_at).toLocaleString()}</span>
                  )}
                  {meeting.status === 'active' && (
                    <span className="ml-2 inline-flex items-center gap-1 text-green-600">
                      <Radio className="h-3 w-3" /> recording &amp; transcribing automatically
                    </span>
                  )}
                  {meeting.status === 'ended' && (
                    <span className="ml-2 inline-flex items-center gap-1.5 text-amber-600">
                      <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-amber-500" />
                      summarizing transcript...
                    </span>
                  )}
                </div>
              )}
            </div>
          )}

          {meeting.status === 'summarized' && meeting.summary && (
            <div className="space-y-4 rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
              {meeting.started_at && meeting.ended_at && (
                <div className="flex items-center gap-1.5 text-xs text-gray-400">
                  <Clock className="h-3.5 w-3.5" />
                  {new Date(meeting.started_at).toLocaleString()} · {new Date(meeting.ended_at).toLocaleString()}
                </div>
              )}

              <div>
                <label className="mb-1 block text-xs font-medium text-gray-500">Executive Summary</label>
                <p className="rounded-lg bg-blue-50 p-3 text-sm text-blue-900">
                  {meeting.summary.executive_summary}
                </p>
              </div>

              <div>
                <label className="mb-2 flex items-center gap-1.5 text-xs font-medium text-gray-500">
                  <ListChecks className="h-3.5 w-3.5" /> Key Action Items
                </label>
                {(meeting.summary.action_items ?? []).length > 0 ? (
                  <ul className="space-y-1.5">
                    {meeting.summary.action_items.map((item, i) => (
                      <li key={i} className="flex items-start gap-2 text-sm text-gray-700">
                        <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-amber-400" />
                        <span>
                          {item.description}
                          {item.owner_hint && (
                            <span className="ml-1.5 text-xs text-gray-400">({item.owner_hint})</span>
                          )}
                        </span>
                      </li>
                    ))}
                  </ul>
                ) : (
                  <p className="text-sm text-gray-400">No action items identified.</p>
                )}
              </div>

              <div>
                <label className="mb-2 flex items-center gap-1.5 text-xs font-medium text-gray-500">
                  <Users className="h-3.5 w-3.5" /> Participants
                </label>
                {(meeting.attendance ?? []).length > 0 ? (
                  <div className="flex flex-wrap gap-2">
                    {(meeting.attendance ?? []).map((p, i) => (
                      <span
                        key={i}
                        className="inline-flex items-center gap-1.5 rounded-full bg-gray-100 py-0.5 pl-0.5 pr-2.5 text-xs font-medium text-gray-700"
                      >
                        <span className="flex h-5 w-5 items-center justify-center rounded-full bg-gray-300 text-[10px] font-semibold text-gray-700">
                          {(p.name || p.email || '?').charAt(0).toUpperCase()}
                        </span>
                        {p.name || p.email || 'Unknown'}
                      </span>
                    ))}
                  </div>
                ) : (
                  <p className="text-sm text-gray-400">No attendance recorded.</p>
                )}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
