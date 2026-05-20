import { useQuery } from "@tanstack/react-query";
import { getEventReadiness } from "@/lib/api/client";

export const eventReadinessQueryKey = ["event-readiness"] as const;

export function useEventReadinessQuery() {
  return useQuery({
    queryKey: eventReadinessQueryKey,
    queryFn: getEventReadiness,
  });
}
