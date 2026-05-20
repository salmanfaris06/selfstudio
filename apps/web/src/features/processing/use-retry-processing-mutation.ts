import { useMutation, useQueryClient } from "@tanstack/react-query";
import { retryPhotoProcessing } from "@/lib/api/client";
import { processingQueueQueryKey } from "./use-processing-queue-query";

export function useRetryProcessingMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (photoId: string) => retryPhotoProcessing(photoId),
    onSettled: () => queryClient.invalidateQueries({ queryKey: processingQueueQueryKey }),
  });
}
