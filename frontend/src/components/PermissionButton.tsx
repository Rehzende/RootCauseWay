import type { ButtonHTMLAttributes } from 'react';
import { useAuth } from '@/hooks/useAuth';

interface PermissionButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  resource: string;
  action: string;
  /** Message shown as a tooltip when the action is disallowed. */
  deniedTitle?: string;
}

// Drop-in <button> replacement for actions that should stay visible but
// become unclickable when the user lacks permission -- e.g. Edit. This is
// deliberately NOT PermissionGate (which unmounts children entirely): the
// UI convention here is "hide what you can't see, disable what you can see
// but can't do" -- delete uses PermissionGate, edit uses this.
export function PermissionButton({
  resource,
  action,
  disabled,
  className,
  title,
  deniedTitle = "You don't have permission to do this",
  ...rest
}: PermissionButtonProps) {
  const { hasPermission } = useAuth();
  const allowed = hasPermission(resource, action);

  return (
    <button
      {...rest}
      disabled={disabled || !allowed}
      title={allowed ? title : deniedTitle}
      aria-disabled={!allowed}
      className={`${className ?? ''} ${!allowed ? 'cursor-not-allowed opacity-40 hover:!bg-transparent hover:!text-inherit' : ''}`}
    />
  );
}
