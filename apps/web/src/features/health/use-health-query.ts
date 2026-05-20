import { useQuery } from "@tanstack/react-query";
import { getHealth } from "@/lib/api/client";

export const healthQueryKey = ["health"] as const;

export function useHealthQuery() {
  return useQuery({
    queryKey: healthQueryKey,
    queryFn: getHealth,
    refetchInterval: 30_000,
  });
}
