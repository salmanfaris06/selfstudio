import { useMutation, useQueryClient } from "@tanstack/react-query";
import { activityQueryKey } from "@/features/activity/use-activity-query";
import { eventReadinessQueryKey } from "@/features/readiness/use-event-readiness-query";
import { stationReadinessQueryKey } from "@/features/stations/use-station-readiness-query";
import { stationsQueryKey } from "@/features/stations/use-stations-query";
import { endSession } from "@/lib/api/client";
import { sessionsQueryKey } from "./use-sessions-query";

export function useEndSessionMutation(stationId: string, sessionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => endSession(sessionId, { reason: "manual" }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: sessionsQueryKey });
      void queryClient.invalidateQueries({ queryKey: stationsQueryKey });
      void queryClient.invalidateQueries({ queryKey: stationReadinessQueryKey(stationId) });
      void queryClient.invalidateQueries({ queryKey: eventReadinessQueryKey });
      void queryClient.invalidateQueries({ queryKey: activityQueryKey("") });
    },
  });
}
