import { useQuery } from "@tanstack/react-query";
import { getSessionDetail } from "@/lib/api/client";

export function sessionDetailQueryKey(sessionId: string) {
  return ["sessions", sessionId] as const;
}

export function useSessionDetailQuery(sessionId: string | null) {
  return useQuery({
    queryKey: sessionDetailQueryKey(sessionId ?? ""),
    queryFn: () => getSessionDetail(sessionId ?? ""),
    enabled: Boolean(sessionId),
  });
}
