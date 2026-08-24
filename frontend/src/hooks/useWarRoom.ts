import { useQuery } from '@tanstack/react-query';
import { useToastMutation } from '@/hooks/useToastMutation';
import { getWarRoom, createWarRoom, endWarRoom } from '@/services/api';
import type { WarRoomMeeting } from '@/types/api';

export function useWarRoom(incidentId: string) {
  return useQuery({
    queryKey: ['warroom', incidentId],
    queryFn: () => getWarRoom(incidentId),
    enabled: !!incidentId,
    // 'summarized' is the only real terminal status. 'ended' still needs
    // polling: EndWarRoom flips status to 'ended' synchronously, then
    // agent-service's WarRoomConsumer summarizes the transcript
    // asynchronously (seconds to a couple minutes) and PATCHes the
    // meeting to 'summarized' -- stopping polling at 'ended' meant the
    // panel never picked up the summary without a manual page reload.
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      return status && status !== 'summarized' ? 10000 : false;
    },
  });
}

export function useStartWarRoom(incidentId: string) {
  return useToastMutation<WarRoomMeeting, void>({
    mutationFn: () => createWarRoom(incidentId),
    successTitle: 'War room started',
    errorTitle: 'Failed to start war room',
    invalidateKeys: [['warroom', incidentId]],
  });
}

export function useEndWarRoom(incidentId: string) {
  return useToastMutation<WarRoomMeeting, string>({
    mutationFn: (meetingId: string) => endWarRoom(meetingId),
    successTitle: 'War room ended',
    errorTitle: 'Failed to end war room',
    invalidateKeys: [['warroom', incidentId]],
  });
}
