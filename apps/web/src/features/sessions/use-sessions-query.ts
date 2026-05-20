import { useQuery } from "@tanstack/react-query";
import { getSessions } from "@/lib/api/client";

export const sessionsQueryKey = ["sessions"] as const;

export function useSessionsQuery() {
  return useQuery({
    queryKey: sessionsQueryKey,
    queryFn: getSessions,
    refetchInterval: 15_000,
  });
}
