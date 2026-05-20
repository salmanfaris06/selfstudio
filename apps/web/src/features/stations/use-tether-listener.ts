import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { activityQueryKey } from "@/features/activity/use-activity-query";
import { getTetherListener, retryTetherListener, startTetherListener, stopTetherListener, updateTetherListenerSettings } from "@/lib/api/client";
import { stationReadinessQueryKey } from "./use-station-readiness-query";
import { stationsQueryKey } from "./use-stations-query";

export const tetherListenerQueryKey = (stationId: string) => ["tether-listener", stationId] as const;

export function useTetherListenerQuery(stationId: string) {
  return useQuery({
    queryKey: tetherListenerQueryKey(stationId),
    queryFn: () => getTetherListener(stationId),
    refetchInterval: 10_000,
  });
}

export function useStartTetherListenerMutation(stationId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => startTetherListener(stationId),
    onSuccess: (data) => {
      queryClient.setQueryData(tetherListenerQueryKey(stationId), data);
      void queryClient.invalidateQueries({ queryKey: stationReadinessQueryKey(stationId) });
      void queryClient.invalidateQueries({ queryKey: stationsQueryKey });
      void queryClient.invalidateQueries({ queryKey: activityQueryKey("") });
    },
  });
}

export function useRetryTetherListenerMutation(stationId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => retryTetherListener(stationId),
    onSuccess: (data) => {
      queryClient.setQueryData(tetherListenerQueryKey(stationId), data);
      void queryClient.invalidateQueries({ queryKey: stationReadinessQueryKey(stationId) });
      void queryClient.invalidateQueries({ queryKey: stationsQueryKey });
      void queryClient.invalidateQueries({ queryKey: activityQueryKey("") });
    },
  });
}

export function useUpdateTetherSettingsMutation(stationId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (autoRestartEnabled: boolean) => updateTetherListenerSettings(stationId, autoRestartEnabled),
    onSuccess: (data) => {
      queryClient.setQueryData(tetherListenerQueryKey(stationId), data);
      void queryClient.invalidateQueries({ queryKey: activityQueryKey("") });
    },
  });
}

export function useStopTetherListenerMutation(stationId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => stopTetherListener(stationId),
    onSuccess: (data) => {
      queryClient.setQueryData(tetherListenerQueryKey(stationId), data);
      void queryClient.invalidateQueries({ queryKey: stationReadinessQueryKey(stationId) });
      void queryClient.invalidateQueries({ queryKey: stationsQueryKey });
      void queryClient.invalidateQueries({ queryKey: activityQueryKey("") });
    },
  });
}
