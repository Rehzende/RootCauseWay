import { useMutation, useQueryClient, type UseMutationOptions } from '@tanstack/react-query';
import { useToast } from '@/components/Toast';

interface ToastMutationOptions<TData, TError, TVariables> extends Omit<UseMutationOptions<TData, TError, TVariables>, 'onSuccess' | 'onError'> {
  successTitle: string;
  errorTitle: string;
  invalidateKeys?: string[][];
  onSuccess?: (data: TData, variables: TVariables) => void;
  onError?: (error: TError) => void;
}

export function useToastMutation<TData = unknown, TVariables = void>(
  opts: ToastMutationOptions<TData, any, TVariables>
) {
  const { addToast } = useToast();
  const queryClient = useQueryClient();

  return useMutation({
    ...opts,
    onSuccess: (data, variables) => {
      addToast({ type: 'success', title: opts.successTitle });
      opts.invalidateKeys?.forEach((key) => queryClient.invalidateQueries({ queryKey: key }));
      opts.onSuccess?.(data, variables);
    },
    onError: (error: any) => {
      addToast({
        type: 'error',
        title: opts.errorTitle,
        message: error?.response?.data?.error || error?.message || 'Unknown error',
      });
      opts.onError?.(error);
    },
  });
}
