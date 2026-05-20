import { useMutation, useQueryClient } from "@tanstack/react-query";
import { runEventReadinessCheck } from "@/lib/api/client";
import { eventReadinessQueryKey } from "./use-event-readiness-query";

export function useRunEventReadinessCheckMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: runEventReadinessCheck,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: eventReadinessQueryKey });
    },
  });
}
