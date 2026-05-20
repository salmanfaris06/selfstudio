import { useMutation, useQueryClient } from "@tanstack/react-query";
import { updateStation, type UpdateStationRequest } from "@/lib/api/client";
import { stationReadinessQueryKey } from "./use-station-readiness-query";
import { stationsQueryKey } from "./use-stations-query";

export function useUpdateStationMutation(stationId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (request: UpdateStationRequest) => updateStation(stationId, request),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: stationsQueryKey });
      void queryClient.invalidateQueries({ queryKey: stationReadinessQueryKey(stationId) });
    },
  });
}
