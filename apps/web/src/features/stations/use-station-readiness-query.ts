import { useQuery } from "@tanstack/react-query";
import { getStationReadiness } from "@/lib/api/client";

export const stationReadinessQueryKey = (stationId: string) => ["stations", stationId, "readiness"] as const;

export function useStationReadinessQuery(stationId: string) {
  return useQuery({
    queryKey: stationReadinessQueryKey(stationId),
    queryFn: () => getStationReadiness(stationId),
  });
}
