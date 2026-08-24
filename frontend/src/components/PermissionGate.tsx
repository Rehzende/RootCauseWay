import type { ReactNode } from 'react';
import { useAuth } from '@/hooks/useAuth';

interface PermissionGateProps {
  resource: string;
  action: string;
  children: ReactNode;
  fallback?: ReactNode;
}

export function PermissionGate({ resource, action, children, fallback }: PermissionGateProps) {
  const { hasPermission } = useAuth();
  if (hasPermission(resource, action)) return <>{children}</>;
  return fallback ? <>{fallback}</> : null;
}

export function AccessDeniedPage() {
  return (
    <div className="flex h-full items-center justify-center p-8">
      <div className="text-center">
        <h1 className="text-2xl font-bold text-gray-900">Access Denied</h1>
        <p className="mt-2 text-sm text-gray-500">You do not have permission to view this page.</p>
      </div>
    </div>
  );
}
