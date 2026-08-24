interface FiveWhyEntry {
  why: string;
  answer: string;
}

interface FiveWhysProps {
  whys: (string | FiveWhyEntry)[];
}

export function FiveWhys({ whys }: FiveWhysProps) {
  if (!whys || whys.length === 0) {
    return <p className="text-sm italic text-gray-400">No 5 Whys analysis yet.</p>;
  }

  return (
    <div className="space-y-0">
      {whys.map((item, idx) => {
        const isObj = typeof item === 'object' && item !== null;
        const question = isObj ? (item as FiveWhyEntry).why : String(item);
        const answer = isObj ? (item as FiveWhyEntry).answer : undefined;

        return (
          <div key={idx} className="relative flex items-start gap-3 pb-4">
            {idx < whys.length - 1 && (
              <span className="absolute left-4 top-8 h-full w-0.5 bg-red-200" />
            )}
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-red-100 text-sm font-bold text-red-700">
              {idx + 1}
            </div>
            <div className="pt-1 space-y-1">
              <p className="text-xs font-medium uppercase text-red-500">Why #{idx + 1}</p>
              <p className="text-sm font-medium text-gray-800">{question}</p>
              {answer && (
                <p className="text-sm text-gray-600 italic">{answer}</p>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}
