import { useMutation, useQueryClient } from "@tanstack/react-query";
import { runStationReadinessCheck } from "@/lib/api/client";
import { stationsQueryKey } from "./use-stations-query";
import { stationReadinessQueryKey } from "./use-station-readiness-query";

export function useRunReadinessCheckMutation(stationId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () => runStationReadinessCheck(stationId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: stationsQueryKey });
      void queryClient.invalidateQueries({ queryKey: stationReadinessQueryKey(stationId) });
    },
  });
}
