import { useMutation, useQueryClient } from "@tanstack/react-query";
import { eventReadinessQueryKey } from "@/features/readiness/use-event-readiness-query";
import { refreshStationHealth } from "@/lib/api/client";
import { stationReadinessQueryKey } from "./use-station-readiness-query";
import { stationsQueryKey } from "./use-stations-query";

export function useRefreshStationHealthMutation(stationId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => refreshStationHealth(stationId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: stationsQueryKey });
      void queryClient.invalidateQueries({ queryKey: stationReadinessQueryKey(stationId) });
      void queryClient.invalidateQueries({ queryKey: eventReadinessQueryKey });
    },
  });
}
