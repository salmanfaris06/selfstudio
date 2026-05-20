import { useMutation, useQueryClient } from "@tanstack/react-query";
import { retrySessionUploads } from "@/lib/api/client";

export function useRetryCloudUploadMutation(sessionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => retrySessionUploads(sessionId),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ["session-detail", sessionId],
        }),
        queryClient.invalidateQueries({
          queryKey: ["session-uploads", sessionId],
        }),
        queryClient.invalidateQueries({ queryKey: ["activity"] }),
      ]);
    },
  });
}
