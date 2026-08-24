import { useQuery } from '@tanstack/react-query';
import { useToastMutation } from '@/hooks/useToastMutation';
import {
  listRetentionPolicies,
  createRetentionPolicy,
  updateRetentionPolicy,
  deleteRetentionPolicy,
  runRetentionSweep,
} from '@/services/api';
import type {
  RetentionPolicy,
  CreateRetentionPolicyRequest,
  UpdateRetentionPolicyRequest,
  RetentionSweepSummary,
} from '@/types/api';

export function useRetentionPolicies() {
  return useQuery({ queryKey: ['retention-policies'], queryFn: listRetentionPolicies });
}

export function useCreateRetentionPolicy() {
  return useToastMutation<RetentionPolicy, CreateRetentionPolicyRequest>({
    mutationFn: createRetentionPolicy,
    successTitle: 'Retention policy created',
    errorTitle: 'Failed to create retention policy',
    invalidateKeys: [['retention-policies']],
  });
}

export function useUpdateRetentionPolicy() {
  return useToastMutation<RetentionPolicy, { id: string; data: UpdateRetentionPolicyRequest }>({
    mutationFn: ({ id, data }) => updateRetentionPolicy(id, data),
    successTitle: 'Retention policy updated',
    errorTitle: 'Failed to update retention policy',
    invalidateKeys: [['retention-policies']],
  });
}

export function useDeleteRetentionPolicy() {
  return useToastMutation<unknown, string>({
    mutationFn: deleteRetentionPolicy,
    successTitle: 'Retention policy deleted',
    errorTitle: 'Failed to delete retention policy',
    invalidateKeys: [['retention-policies']],
  });
}

export function useRunRetentionSweep(onSuccess?: (summary: RetentionSweepSummary) => void) {
  return useToastMutation<RetentionSweepSummary, void>({
    mutationFn: runRetentionSweep,
    successTitle: 'Retention sweep completed',
    errorTitle: 'Retention sweep failed',
    invalidateKeys: [['retention-policies']],
    onSuccess,
  });
}
