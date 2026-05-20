import { useMutation, useQueryClient } from "@tanstack/react-query";
import { runCameraTestCapture } from "@/lib/api/client";

export function useCameraTestCaptureMutation(stationId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => runCameraTestCapture(stationId),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["station-readiness", stationId] }),
        queryClient.invalidateQueries({ queryKey: ["event-readiness"] }),
        queryClient.invalidateQueries({ queryKey: ["tether-listener", stationId] }),
        queryClient.invalidateQueries({ queryKey: ["stations"] }),
      ]);
    },
  });
}
