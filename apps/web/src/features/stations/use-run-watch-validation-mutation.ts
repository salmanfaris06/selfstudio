import { useMutation, useQueryClient } from "@tanstack/react-query";
import { eventReadinessQueryKey } from "@/features/readiness/use-event-readiness-query";
import { runWatchValidation } from "@/lib/api/client";
import { stationReadinessQueryKey } from "./use-station-readiness-query";
import { stationsQueryKey } from "./use-stations-query";

export function useRunWatchValidationMutation(stationId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => runWatchValidation(stationId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: stationsQueryKey });
      void queryClient.invalidateQueries({ queryKey: stationReadinessQueryKey(stationId) });
      void queryClient.invalidateQueries({ queryKey: eventReadinessQueryKey });
    },
  });
}
