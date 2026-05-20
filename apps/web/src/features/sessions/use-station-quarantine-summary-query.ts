import { useQuery } from "@tanstack/react-query";
import { getStationQuarantineSummary } from "@/lib/api/client";

export function stationQuarantineSummaryQueryKey(stationId: string) {
  return ["stations", stationId, "quarantine-summary"] as const;
}

export function useStationQuarantineSummaryQuery(stationId: string | null) {
  return useQuery({
    queryKey: stationQuarantineSummaryQueryKey(stationId ?? ""),
    queryFn: () => getStationQuarantineSummary(stationId ?? ""),
    enabled: Boolean(stationId),
  });
}
