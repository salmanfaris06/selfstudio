import { useMutation, useQueryClient } from "@tanstack/react-query";
import { activityQueryKey } from "@/features/activity/use-activity-query";
import { resolveSessionCloudTarget } from "@/lib/api/client";
import { sessionDetailQueryKey } from "./use-session-detail-query";
import { sessionsQueryKey } from "./use-sessions-query";

export function useResolveCloudTargetMutation(sessionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => resolveSessionCloudTarget(sessionId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: sessionDetailQueryKey(sessionId) });
      void queryClient.invalidateQueries({ queryKey: sessionsQueryKey });
      void queryClient.invalidateQueries({ queryKey: activityQueryKey("") });
    },
  });
}
