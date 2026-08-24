import { useState, useEffect, useMemo } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { listIncidents, listSoftware } from '@/services/api';
import { DataTable, type Column } from '@/components/DataTable';
import { SeverityBadge } from '@/components/SeverityBadge';
import { StatusBadge } from '@/components/StatusBadge';
import { SkeletonTable } from '@/components/Skeleton';
import { NoIncidents } from '@/components/EmptyState';
import { useWebSocket } from '@/hooks/useWebSocket';
import { useToast } from '@/components/Toast';
import { formatIncidentCode } from '@/lib/incident';
import type { Incident, Severity, IncidentStatus } from '@/types/api';

type DateRange = '24h' | '7d' | '30d' | 'all';

function getFromDate(range: DateRange): string | undefined {
  if (range === 'all') return undefined;
  const now = new Date();
  switch (range) {
    case '24h': now.setHours(now.getHours() - 24); break;
    case '7d': now.setDate(now.getDate() - 7); break;
    case '30d': now.setDate(now.getDate() - 30); break;
  }
  return now.toISOString();
}

export function IncidentsPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const [statusFilter, setStatusFilter] = useState('');
  const [severityFilter, setSeverityFilter] = useState('');
  const [softwareFilter, setSoftwareFilter] = useState('');
  const [dateRange, setDateRange] = useState<DateRange>('all');
  const [searchText, setSearchText] = useState('');
  const { lastEvent } = useWebSocket(['incident.created']);
  const { addToast } = useToast();

  useEffect(() => {
    if (!lastEvent || lastEvent.type !== 'incident.created') return;
    queryClient.invalidateQueries({ queryKey: ['incidents'] });
    const eventData = lastEvent.data as { title?: string; incident_number?: number } | undefined;
    const code = eventData?.incident_number ? `${formatIncidentCode(eventData.incident_number)} - ` : '';
    addToast({
      type: 'info',
      title: `New incident: ${code}${eventData?.title ?? 'Unknown'}`,
      action: lastEvent.incident_id
        ? { label: 'View', href: `/incidents/${lastEvent.incident_id}` }
        : undefined,
    });
  }, [lastEvent]); // eslint-disable-line react-hooks/exhaustive-deps

  const fromDate = getFromDate(dateRange);

  const { data, isLoading } = useQuery({
    queryKey: ['incidents', page, statusFilter, severityFilter, softwareFilter, fromDate],
    queryFn: () => listIncidents({
      page,
      ...(statusFilter ? { status: statusFilter } : {}),
      ...(severityFilter ? { severity: severityFilter } : {}),
      ...(softwareFilter ? { software_id: softwareFilter } : {}),
      ...(fromDate ? { from: fromDate } : {}),
    }),
  });

  const { data: softwareData } = useQuery({
    queryKey: ['software', 'filter-list'],
    queryFn: () => listSoftware(1, 100),
  });

  const softwareList = softwareData?.data ?? [];

  const filteredData = useMemo(() => {
    const items = data?.data ?? [];
    if (!searchText.trim()) return items;
    const lower = searchText.toLowerCase();
    return items.filter((i: Incident) => i.title.toLowerCase().includes(lower));
  }, [data?.data, searchText]);

  const columns: Column<Incident>[] = [
    { key: 'title', header: 'Title', render: (i) => (
      <span className="font-medium text-gray-900">
        <span className="text-gray-500 font-normal">{formatIncidentCode(i.incident_number)}</span> - {i.title}
      </span>
    )},
    { key: 'severity', header: 'Severity', render: (i) => <SeverityBadge severity={i.severity} /> },
    { key: 'status', header: 'Status', render: (i) => <StatusBadge status={i.status} /> },
    { key: 'created', header: 'Created', render: (i) => (
      <span className="text-sm text-gray-500">{new Date(i.created_at).toLocaleDateString()}</span>
    )},
  ];

  const statuses: IncidentStatus[] = ['open', 'investigating', 'mitigated', 'resolved', 'closed'];
  const severities: Severity[] = ['critical', 'high', 'medium', 'low'];
  const dateRanges: { value: DateRange; label: string }[] = [
    { value: '24h', label: 'Last 24h' },
    { value: '7d', label: 'Last 7d' },
    { value: '30d', label: 'Last 30d' },
    { value: 'all', label: 'All time' },
  ];

  return (
    <div className="p-8">
      <h1 className="text-2xl font-bold text-gray-900">Incidents</h1>
      <p className="mt-1 text-sm text-gray-500">Track and investigate incidents</p>

      {/* Existing filters row */}
      <div className="mt-4 flex flex-wrap gap-3">
        <select
          value={statusFilter}
          onChange={(e) => { setStatusFilter(e.target.value); setPage(1); }}
          className="rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none"
        >
          <option value="">All statuses</option>
          {statuses.map((s) => <option key={s} value={s}>{s}</option>)}
        </select>
        <select
          value={severityFilter}
          onChange={(e) => { setSeverityFilter(e.target.value); setPage(1); }}
          className="rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none"
        >
          <option value="">All severities</option>
          {severities.map((s) => <option key={s} value={s}>{s}</option>)}
        </select>

        {/* Software filter */}
        <select
          value={softwareFilter}
          onChange={(e) => { setSoftwareFilter(e.target.value); setPage(1); }}
          className="rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none"
        >
          <option value="">All software</option>
          {softwareList.map((sw) => <option key={sw.id} value={sw.id}>{sw.name}</option>)}
        </select>

        {/* Search */}
        <input
          type="text"
          value={searchText}
          onChange={(e) => setSearchText(e.target.value)}
          placeholder="Search by title..."
          className="rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none"
        />
      </div>

      {/* Date range buttons */}
      <div className="mt-3 flex gap-1">
        {dateRanges.map((dr) => (
          <button
            key={dr.value}
            onClick={() => { setDateRange(dr.value); setPage(1); }}
            className={`rounded-md px-3 py-1.5 text-sm font-medium ${
              dateRange === dr.value
                ? 'bg-blue-600 text-white'
                : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
            }`}
          >
            {dr.label}
          </button>
        ))}
      </div>

      <div className="mt-4">
        {isLoading ? (
          <SkeletonTable rows={8} cols={4} />
        ) : filteredData.length === 0 ? (
          <NoIncidents />
        ) : (
          <DataTable
            columns={columns}
            data={filteredData}
            total={searchText.trim() ? filteredData.length : (data?.total ?? 0)}
            page={searchText.trim() ? 1 : page}
            perPage={data?.per_page ?? 20}
            onPageChange={setPage}
            keyExtractor={(i) => i.id}
            onRowClick={(i) => navigate(`/incidents/${i.id}`)}
          />
        )}
      </div>
    </div>
  );
}
