import { AlertTriangle, Box, Bot } from 'lucide-react';
import type { ReactNode } from 'react';

interface EmptyStateProps {
  icon?: ReactNode;
  title: string;
  description?: string;
  action?: ReactNode;
}

export function EmptyState({ icon, title, description, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      {icon && (
        <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-gray-100">
          {icon}
        </div>
      )}
      <h3 className="text-base font-semibold text-gray-900">{title}</h3>
      {description && (
        <p className="mt-1 max-w-sm text-sm text-gray-500">{description}</p>
      )}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}

export function NoIncidents() {
  return (
    <EmptyState
      icon={<AlertTriangle className="h-7 w-7 text-gray-400" />}
      title="No incidents yet"
      description="Configure a webhook to start monitoring your services for incidents."
    />
  );
}

export function NoSoftware() {
  return (
    <EmptyState
      icon={<Box className="h-7 w-7 text-gray-400" />}
      title="No services registered"
      description="Add your first service to begin incident monitoring."
    />
  );
}

export function NoAgents() {
  return (
    <EmptyState
      icon={<Bot className="h-7 w-7 text-gray-400" />}
      title="No agents configured"
      description="Set up AI agents to automatically investigate incidents."
    />
  );
}
