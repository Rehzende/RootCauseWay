import { useQuery } from '@tanstack/react-query';
import { useToastMutation } from '@/hooks/useToastMutation';
import {
  listSLODefinitions,
  createSLODefinition,
  updateSLODefinition,
  deleteSLODefinition,
  getSLOStatus,
  getSoftwareSLOStatus,
} from '@/services/api';
import type { SLODefinition, CreateSLODefinitionRequest, UpdateSLODefinitionRequest } from '@/types/api';

export function useSLODefinitions() {
  return useQuery({ queryKey: ['slo-definitions'], queryFn: listSLODefinitions });
}

export function useSLOStatus(id: string | undefined) {
  return useQuery({
    queryKey: ['slo-status', id],
    queryFn: () => getSLOStatus(id!),
    enabled: !!id,
  });
}

export function useSoftwareSLOStatus(softwareId: string | undefined) {
  return useQuery({
    queryKey: ['software-slo-status', softwareId],
    queryFn: () => getSoftwareSLOStatus(softwareId!),
    enabled: !!softwareId,
  });
}

export function useCreateSLODefinition() {
  return useToastMutation<SLODefinition, CreateSLODefinitionRequest>({
    mutationFn: createSLODefinition,
    successTitle: 'SLO created',
    errorTitle: 'Failed to create SLO',
    invalidateKeys: [['slo-definitions']],
  });
}

export function useUpdateSLODefinition() {
  return useToastMutation<SLODefinition, { id: string; data: UpdateSLODefinitionRequest }>({
    mutationFn: ({ id, data }) => updateSLODefinition(id, data),
    successTitle: 'SLO updated',
    errorTitle: 'Failed to update SLO',
    invalidateKeys: [['slo-definitions']],
  });
}

export function useDeleteSLODefinition() {
  return useToastMutation<unknown, string>({
    mutationFn: deleteSLODefinition,
    successTitle: 'SLO deleted',
    errorTitle: 'Failed to delete SLO',
    invalidateKeys: [['slo-definitions']],
  });
}
