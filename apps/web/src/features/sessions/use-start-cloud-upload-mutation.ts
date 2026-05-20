import { useMutation, useQueryClient } from "@tanstack/react-query";
import { startSessionUploads } from "@/lib/api/client";

export function useStartCloudUploadMutation(sessionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => startSessionUploads(sessionId),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["session-detail", sessionId] }),
        queryClient.invalidateQueries({ queryKey: ["session-uploads", sessionId] }),
        queryClient.invalidateQueries({ queryKey: ["activity"] }),
      ]);
    },
  });
}
