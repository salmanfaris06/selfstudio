import { useMutation, useQueryClient } from "@tanstack/react-query";
import { activityQueryKey } from "@/features/activity/use-activity-query";
import { eventReadinessQueryKey } from "@/features/readiness/use-event-readiness-query";
import { startSession, type StartSessionRequest } from "@/lib/api/client";
import { stationReadinessQueryKey } from "@/features/stations/use-station-readiness-query";
import { stationsQueryKey } from "@/features/stations/use-stations-query";
import { sessionsQueryKey } from "./use-sessions-query";

export function useStartSessionMutation(stationId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (request: StartSessionRequest) => startSession(stationId, request),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: sessionsQueryKey });
      void queryClient.invalidateQueries({ queryKey: stationsQueryKey });
      void queryClient.invalidateQueries({ queryKey: stationReadinessQueryKey(stationId) });
      void queryClient.invalidateQueries({ queryKey: eventReadinessQueryKey });
      void queryClient.invalidateQueries({ queryKey: activityQueryKey("") });
    },
  });
}
