import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from 'react';
import { X } from 'lucide-react';
import { useNavigate } from 'react-router-dom';

export interface ToastData {
  id: string;
  type: 'info' | 'success' | 'warning' | 'error';
  title: string;
  message?: string;
  action?: { label: string; href: string };
  duration?: number;
}

interface ToastContextValue {
  addToast: (toast: Omit<ToastData, 'id'>) => void;
  removeToast: (id: string) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

let toastCounter = 0;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastData[]>([]);

  const addToast = useCallback((toast: Omit<ToastData, 'id'>) => {
    const id = `toast-${++toastCounter}`;
    setToasts((prev) => [...prev, { ...toast, id }]);
  }, []);

  const removeToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  return (
    <ToastContext.Provider value={{ addToast, removeToast }}>
      {children}
      <ToastContainer toasts={toasts} removeToast={removeToast} />
    </ToastContext.Provider>
  );
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error('useToast must be used within ToastProvider');
  return ctx;
}

const typeStyles: Record<ToastData['type'], string> = {
  info: 'border-blue-200 bg-blue-50',
  success: 'border-green-200 bg-green-50',
  warning: 'border-amber-200 bg-amber-50',
  error: 'border-red-200 bg-red-50',
};

const typeTitleStyles: Record<ToastData['type'], string> = {
  info: 'text-blue-800',
  success: 'text-green-800',
  warning: 'text-amber-800',
  error: 'text-red-800',
};

function ToastContainer({ toasts, removeToast }: { toasts: ToastData[]; removeToast: (id: string) => void }) {
  return (
    <div className="fixed right-4 top-4 z-50 flex flex-col gap-2 w-80">
      {toasts.map((toast) => (
        <ToastItem key={toast.id} toast={toast} onDismiss={() => removeToast(toast.id)} />
      ))}
    </div>
  );
}

function ToastItem({ toast, onDismiss }: { toast: ToastData; onDismiss: () => void }) {
  const navigate = useNavigate();

  useEffect(() => {
    const timer = setTimeout(onDismiss, toast.duration ?? 5000);
    return () => clearTimeout(timer);
  }, [toast.duration, onDismiss]);

  return (
    <div
      className={`animate-slide-in rounded-lg border p-3 shadow-lg ${typeStyles[toast.type]}`}
    >
      <div className="flex items-start gap-2">
        <div className="flex-1 min-w-0">
          <p className={`text-sm font-semibold ${typeTitleStyles[toast.type]}`}>{toast.title}</p>
          {toast.message && (
            <p className="mt-0.5 text-xs text-gray-600">{toast.message}</p>
          )}
          {toast.action && (
            <button
              onClick={() => navigate(toast.action!.href)}
              className="mt-1 text-xs font-medium text-blue-600 hover:text-blue-700"
            >
              {toast.action.label}
            </button>
          )}
        </div>
        <button onClick={onDismiss} className="text-gray-400 hover:text-gray-600">
          <X className="h-3.5 w-3.5" />
        </button>
      </div>
    </div>
  );
}
