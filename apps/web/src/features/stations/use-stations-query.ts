import { useQuery } from "@tanstack/react-query";
import { getStations } from "@/lib/api/client";

export const stationsQueryKey = ["stations"] as const;

export function useStationsQuery() {
  return useQuery({
    queryKey: stationsQueryKey,
    queryFn: getStations,
    refetchInterval: 30_000,
  });
}
