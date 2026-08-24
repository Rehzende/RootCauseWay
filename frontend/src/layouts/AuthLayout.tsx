import { Outlet } from 'react-router-dom';
import { AlertTriangle } from 'lucide-react';

export function AuthLayout() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-100">
      <div className="w-full max-w-md">
        <div className="mb-8 flex items-center justify-center gap-2">
          <AlertTriangle className="h-8 w-8 text-blue-600" />
          <span className="text-2xl font-bold text-gray-900">RootCauseway</span>
        </div>
        <Outlet />
      </div>
    </div>
  );
}
