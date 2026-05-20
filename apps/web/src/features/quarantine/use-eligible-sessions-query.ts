import { useQuery } from "@tanstack/react-query";
import { getEligibleQuarantineSessions } from "@/lib/api/client";

export const eligibleSessionsQueryKey = (quarantineId: string | null) => ["quarantine", quarantineId, "eligible-sessions"] as const;

export function useEligibleSessionsQuery(quarantineId: string | null) {
  return useQuery({
    queryKey: eligibleSessionsQueryKey(quarantineId),
    queryFn: () => getEligibleQuarantineSessions(quarantineId ?? ""),
    enabled: Boolean(quarantineId),
  });
}
