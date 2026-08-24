import { useState } from 'react';

export interface PresenceUser {
  id: string;
  name: string;
  avatar_url?: string;
  active: boolean;
}

interface PresenceIndicatorProps {
  users: PresenceUser[];
  maxAvatars?: number;
}

function getInitials(name: string): string {
  return name
    .split(' ')
    .map((w) => w[0])
    .join('')
    .toUpperCase()
    .slice(0, 2);
}

const COLORS = [
  'bg-blue-500',
  'bg-green-500',
  'bg-purple-500',
  'bg-amber-500',
  'bg-pink-500',
  'bg-teal-500',
];

function hashColor(id: string): string {
  let hash = 0;
  for (let i = 0; i < id.length; i++) {
    hash = id.charCodeAt(i) + ((hash << 5) - hash);
  }
  return COLORS[Math.abs(hash) % COLORS.length];
}

export function PresenceIndicator({ users, maxAvatars = 5 }: PresenceIndicatorProps) {
  const [hoveredId, setHoveredId] = useState<string | null>(null);

  if (users.length === 0) return null;

  const visible = users.slice(0, maxAvatars);
  const overflow = users.length - maxAvatars;

  return (
    <div className="flex items-center gap-1">
      <span className="mr-1 text-xs text-gray-500">Viewing:</span>
      <div className="flex -space-x-2">
        {visible.map((user) => (
          <div
            key={user.id}
            className="relative"
            onMouseEnter={() => setHoveredId(user.id)}
            onMouseLeave={() => setHoveredId(null)}
          >
            {user.avatar_url ? (
              <img
                src={user.avatar_url}
                alt={user.name}
                className="h-7 w-7 rounded-full border-2 border-white object-cover"
              />
            ) : (
              <div
                className={`flex h-7 w-7 items-center justify-center rounded-full border-2 border-white text-xs font-medium text-white ${hashColor(user.id)}`}
              >
                {getInitials(user.name)}
              </div>
            )}
            {/* Active dot */}
            {user.active && (
              <span className="absolute bottom-0 right-0 h-2 w-2 rounded-full border border-white bg-green-400" />
            )}
            {/* Tooltip */}
            {hoveredId === user.id && (
              <div className="absolute -top-8 left-1/2 z-20 -translate-x-1/2 whitespace-nowrap rounded bg-gray-900 px-2 py-1 text-xs text-white shadow-lg">
                {user.name} is viewing
              </div>
            )}
          </div>
        ))}
        {overflow > 0 && (
          <div className="flex h-7 w-7 items-center justify-center rounded-full border-2 border-white bg-gray-200 text-xs font-medium text-gray-600">
            +{overflow}
          </div>
        )}
      </div>
    </div>
  );
}
