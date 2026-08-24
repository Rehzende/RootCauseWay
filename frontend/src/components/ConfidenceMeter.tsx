interface ConfidenceMeterProps {
  value: number; // 0-1
  size?: 'sm' | 'md';
}

export function ConfidenceMeter({ value, size = 'md' }: ConfidenceMeterProps) {
  const pct = Math.round(value * 100);
  const color =
    pct >= 80 ? 'bg-green-500' : pct >= 50 ? 'bg-amber-500' : 'bg-red-500';
  const textColor =
    pct >= 80 ? 'text-green-700' : pct >= 50 ? 'text-amber-700' : 'text-red-700';
  const h = size === 'sm' ? 'h-2' : 'h-3';

  return (
    <div className="flex items-center gap-3">
      <div className={`flex-1 rounded-full bg-gray-200 ${h}`}>
        <div
          className={`${h} rounded-full ${color} transition-all duration-500`}
          style={{ width: `${pct}%` }}
        />
      </div>
      <span className={`text-sm font-semibold ${textColor}`}>{pct}%</span>
    </div>
  );
}
