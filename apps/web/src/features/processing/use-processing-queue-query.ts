import { useQuery } from "@tanstack/react-query";
import { getProcessingQueue, type ProcessingQueueFilters } from "@/lib/api/client";

export const processingQueueQueryKey = ["processing", "queue"] as const;

export function processingQueueQueryKeyWithFilters(filters: ProcessingQueueFilters = {}) {
  return [...processingQueueQueryKey, filters] as const;
}

export function useProcessingQueueQuery(filters: ProcessingQueueFilters = {}) {
  return useQuery({
    queryKey: processingQueueQueryKeyWithFilters(filters),
    queryFn: () => getProcessingQueue(filters),
    refetchInterval: 15_000,
  });
}
