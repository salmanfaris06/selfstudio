import { useQuery } from "@tanstack/react-query";
import { getActivityLog } from "@/lib/api/client";

export function activityQueryKey(actionType: string) {
  return ["activity", actionType] as const;
}

export function useActivityQuery(actionType: string) {
  return useQuery({
    queryKey: activityQueryKey(actionType),
    queryFn: () => getActivityLog(actionType || undefined),
    refetchInterval: 30_000,
  });
}
