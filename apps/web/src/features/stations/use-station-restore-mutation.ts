import { useMutation, useQueryClient } from "@tanstack/react-query";
import { restoreStations } from "@/lib/api/client";
import { eventReadinessQueryKey } from "@/features/readiness/use-event-readiness-query";
import { stationReadinessQueryKey } from "./use-station-readiness-query";
import { stationsQueryKey } from "./use-stations-query";

const stationIds = ["station_1", "station_2", "station_3"] as const;

export function useStationRestoreMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (filename: string) => restoreStations(filename),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: stationsQueryKey });
      void queryClient.invalidateQueries({ queryKey: eventReadinessQueryKey });
      for (const stationId of stationIds) {
        void queryClient.invalidateQueries({ queryKey: stationReadinessQueryKey(stationId) });
      }
    },
  });
}
