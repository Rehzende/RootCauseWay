import type { SoftwareCriticality, SoftwareType, DependencyRelation } from '@/types/api';

// Shared styling/labels for the software catalog's criticality tier, entry
// type, and dependency relation -- used by both SoftwarePage (catalog list/
// detail/edit) and IncidentDetailPage (incident's affected-software card),
// kept in one place so they render identically everywhere.

export const criticalityBadge: Record<SoftwareCriticality, string> = {
  critical: 'bg-red-100 text-red-800',
  high: 'bg-orange-100 text-orange-800',
  medium: 'bg-yellow-100 text-yellow-800',
  low: 'bg-gray-100 text-gray-700',
};

export const softwareTypeLabel: Record<SoftwareType, string> = {
  service: 'Service',
  library: 'Library',
  database: 'Database',
  job: 'Job',
  website: 'Website',
  other: 'Other',
};

export const dependencyRelationLabel: Record<DependencyRelation, string> = {
  depends_on: 'depends on',
  uses_api_of: 'uses API of',
  shares_database_with: 'shares database with',
};
