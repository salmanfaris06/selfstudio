import { useMutation, useQueryClient } from "@tanstack/react-query";
import { assignQuarantineItem } from "@/lib/api/client";
import { activityQueryKey } from "@/features/activity/use-activity-query";
import { sessionsQueryKey } from "@/features/sessions/use-sessions-query";
import { stationQuarantineSummaryQueryKey } from "@/features/sessions/use-station-quarantine-summary-query";
import { quarantineQueryKey } from "./use-quarantine-query";

export function useAssignQuarantineMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ quarantineId, sessionId }: { quarantineId: string; sessionId: string }) => assignQuarantineItem(quarantineId, { session_id: sessionId }),
    onSuccess: (data) => {
      void queryClient.invalidateQueries({ queryKey: quarantineQueryKey });
      void queryClient.invalidateQueries({ queryKey: sessionsQueryKey });
      void queryClient.invalidateQueries({ predicate: (query) => Array.isArray(query.queryKey) && query.queryKey[0] === "sessions" });
      void queryClient.invalidateQueries({ queryKey: stationQuarantineSummaryQueryKey(data.quarantine.station_id) });
      void queryClient.invalidateQueries({ queryKey: activityQueryKey("") });
    },
  });
}
