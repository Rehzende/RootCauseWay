import { useState } from 'react';
import { Pencil, Check, CheckCircle, Circle, Download, ChevronDown } from 'lucide-react';
import { useToast } from '@/components/Toast';
import { exportPostmortem } from '@/services/api';
import type { IncidentPostmortem, PostmortemStatus } from '@/types/api';

interface PostmortemViewProps {
  postmortem: IncidentPostmortem | null;
  onUpdate?: (data: Partial<IncidentPostmortem>) => void;
  incidentId?: string;
}

function ExportButton({ incidentId }: { incidentId: string }) {
  const [open, setOpen] = useState(false);
  const [exporting, setExporting] = useState(false);
  const { addToast } = useToast();

  const handleExport = async (format: 'markdown' | 'pdf') => {
    setOpen(false);
    setExporting(true);
    try {
      const { blob, filename } = await exportPostmortem(incidentId, format);
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      a.remove();
      window.URL.revokeObjectURL(url);
    } catch (err: any) {
      addToast({
        type: 'error',
        title: 'Failed to export postmortem',
        message: err?.response?.data?.error || err.message,
      });
    } finally {
      setExporting(false);
    }
  };

  return (
    <div className="relative">
      <button
        onClick={() => setOpen(!open)}
        disabled={exporting}
        className="inline-flex items-center gap-1.5 rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
      >
        <Download className="h-3.5 w-3.5" />
        {exporting ? 'Exporting...' : 'Export'}
        <ChevronDown className="h-3.5 w-3.5" />
      </button>
      {open && (
        <div className="absolute right-0 z-10 mt-1 w-44 rounded-md border border-gray-200 bg-white py-1 shadow-lg">
          <button
            onClick={() => handleExport('markdown')}
            className="block w-full px-4 py-2 text-left text-sm text-gray-700 hover:bg-gray-50"
          >
            Export as Markdown
          </button>
          <button
            onClick={() => handleExport('pdf')}
            className="block w-full px-4 py-2 text-left text-sm text-gray-700 hover:bg-gray-50"
          >
            Export as PDF
          </button>
        </div>
      )}
    </div>
  );
}

const statusFlow: PostmortemStatus[] = ['draft', 'in_review', 'published'];
const statusStyle: Record<PostmortemStatus, string> = {
  draft: 'bg-gray-100 text-gray-700',
  in_review: 'bg-amber-100 text-amber-700',
  published: 'bg-green-100 text-green-700',
};

interface SectionProps {
  title: string;
  content: string;
  onSave?: (val: string) => void;
  bg?: string;
}

function EditableSection({ title, content, onSave, bg = 'bg-gray-50' }: SectionProps) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(content);

  return (
    <div>
      <div className="mb-1 flex items-center justify-between">
        <h4 className="text-sm font-semibold text-gray-900">{title}</h4>
        {onSave && (
          <button
            onClick={() => {
              if (editing) { onSave(value); setEditing(false); }
              else setEditing(true);
            }}
            className="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
          >
            {editing ? <Check className="h-3.5 w-3.5" /> : <Pencil className="h-3.5 w-3.5" />}
          </button>
        )}
      </div>
      {editing ? (
        <textarea
          value={value}
          onChange={(e) => setValue(e.target.value)}
          className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none"
          rows={4}
        />
      ) : (
        <div className={`rounded-lg ${bg} p-3 text-sm text-gray-700 whitespace-pre-wrap`}>
          {content || 'Not written yet.'}
        </div>
      )}
    </div>
  );
}

function ListSection({ title, items, color = 'text-gray-700' }: { title: string; items: string[]; color?: string }) {
  return (
    <div>
      <h4 className="mb-2 text-sm font-semibold text-gray-900">{title}</h4>
      {items.length > 0 ? (
        <ul className="space-y-1.5">
          {items.map((item, i) => (
            <li key={i} className={`flex items-start gap-2 text-sm ${color}`}>
              <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-current opacity-40" />
              {item}
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-sm italic text-gray-400">None documented.</p>
      )}
    </div>
  );
}

export function PostmortemView({ postmortem, onUpdate, incidentId }: PostmortemViewProps) {
  if (!postmortem) {
    return (
      <div className="flex h-64 items-center justify-center text-sm text-gray-400">
        Postmortem not generated yet.
      </div>
    );
  }

  const pm = postmortem;
  const currentIdx = statusFlow.indexOf(pm.status);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <h3 className="text-lg font-semibold text-gray-900">{pm.title || 'Untitled Postmortem'}</h3>
          <span className={`mt-1 inline-block rounded-full px-2.5 py-0.5 text-xs font-medium ${statusStyle[pm.status]}`}>
            {pm.status.replace('_', ' ')}
          </span>
        </div>
        <div className="flex items-center gap-2">
          {incidentId && <ExportButton incidentId={incidentId} />}
          {onUpdate && currentIdx < statusFlow.length - 1 && (
            <button
              onClick={() => onUpdate({ status: statusFlow[currentIdx + 1] })}
              className="rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700"
            >
              {currentIdx === 0 ? 'Submit for Review' : 'Publish'}
            </button>
          )}
        </div>
      </div>

      {/* Status workflow */}
      <div className="flex items-center gap-2">
        {statusFlow.map((s, i) => (
          <div key={s} className="flex items-center gap-2">
            {i <= currentIdx ? (
              <CheckCircle className="h-5 w-5 text-green-500" />
            ) : (
              <Circle className="h-5 w-5 text-gray-300" />
            )}
            <span className={`text-xs font-medium ${i <= currentIdx ? 'text-green-700' : 'text-gray-400'}`}>
              {s.replace('_', ' ')}
            </span>
            {i < statusFlow.length - 1 && <div className="h-px w-8 bg-gray-300" />}
          </div>
        ))}
      </div>

      <hr className="border-gray-200" />

      <EditableSection
        title="Executive Summary"
        content={pm.executive_summary}
        bg="bg-blue-50"
        onSave={onUpdate ? (v) => onUpdate({ executive_summary: v }) : undefined}
      />
      <EditableSection
        title="Timeline Narrative"
        content={pm.incident_timeline_narrative}
        onSave={onUpdate ? (v) => onUpdate({ incident_timeline_narrative: v }) : undefined}
      />
      <EditableSection
        title="Root Cause Detail"
        content={pm.root_cause_detail}
        bg="bg-red-50"
        onSave={onUpdate ? (v) => onUpdate({ root_cause_detail: v }) : undefined}
      />
      <EditableSection
        title="Impact Detail"
        content={pm.impact_detail}
        bg="bg-amber-50"
        onSave={onUpdate ? (v) => onUpdate({ impact_detail: v }) : undefined}
      />

      <ListSection title="Lessons Learned" items={pm.lessons_learned} />

      {/* Action Items as checklist */}
      <div>
        <h4 className="mb-2 text-sm font-semibold text-gray-900">Action Items</h4>
        {pm.action_items.length > 0 ? (
          <div className="space-y-2">
            {pm.action_items.map((item, i) => (
              <div key={i} className="flex items-start gap-3 rounded-lg border border-gray-200 p-3">
                <input
                  type="checkbox"
                  checked={item.completed}
                  readOnly={!onUpdate}
                  onChange={() => {
                    if (!onUpdate) return;
                    const updated = [...pm.action_items];
                    updated[i] = { ...item, completed: !item.completed };
                    onUpdate({ action_items: updated });
                  }}
                  className="mt-0.5 h-4 w-4 rounded border-gray-300 text-blue-600"
                />
                <div className="flex-1">
                  <p className={`text-sm ${item.completed ? 'text-gray-400 line-through' : 'text-gray-700'}`}>
                    {item.description}
                  </p>
                  <div className="mt-1 flex gap-3 text-xs text-gray-400">
                    <span>Owner: {item.owner}</span>
                    <span>Due: {item.due_date ? new Date(item.due_date).toLocaleDateString() : '--'}</span>
                  </div>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm italic text-gray-400">No action items.</p>
        )}
      </div>

      <div className="grid grid-cols-2 gap-6">
        <ListSection title="What Went Well" items={pm.what_went_well} color="text-green-700" />
        <ListSection title="What Went Wrong" items={pm.what_went_wrong} color="text-red-700" />
      </div>

      <ListSection title="Prevention Measures" items={pm.prevention_measures} />
    </div>
  );
}
