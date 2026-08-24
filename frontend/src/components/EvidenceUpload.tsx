import { useRef, useState } from 'react';
import { Upload, Loader2 } from 'lucide-react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { uploadEvidence } from '@/services/api';

interface EvidenceUploadProps {
  incidentId: string;
}

export function EvidenceUpload({ incidentId }: EvidenceUploadProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const queryClient = useQueryClient();
  const [dragOver, setDragOver] = useState(false);

  const mutation = useMutation({
    mutationFn: (file: File) => uploadEvidence(incidentId, file),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['incident-full', incidentId] });
    },
  });

  const handleFiles = (files: FileList | null) => {
    if (!files) return;
    Array.from(files).forEach((f) => mutation.mutate(f));
  };

  return (
    <div
      onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
      onDragLeave={() => setDragOver(false)}
      onDrop={(e) => { e.preventDefault(); setDragOver(false); handleFiles(e.dataTransfer.files); }}
      className={`flex cursor-pointer items-center justify-center gap-2 rounded-lg border-2 border-dashed p-4 text-sm transition-colors ${
        dragOver ? 'border-blue-400 bg-blue-50 text-blue-600' : 'border-gray-300 text-gray-500 hover:border-gray-400'
      }`}
      onClick={() => inputRef.current?.click()}
    >
      {mutation.isPending ? (
        <Loader2 className="h-4 w-4 animate-spin" />
      ) : (
        <Upload className="h-4 w-4" />
      )}
      <span>{mutation.isPending ? 'Uploading...' : 'Drop files or click to upload evidence'}</span>
      <input
        ref={inputRef}
        type="file"
        className="hidden"
        multiple
        onChange={(e) => handleFiles(e.target.files)}
      />
    </div>
  );
}
