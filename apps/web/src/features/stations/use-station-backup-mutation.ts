import { useMutation } from "@tanstack/react-query";
import { backupStations } from "@/lib/api/client";

export function useStationBackupMutation() {
  return useMutation({ mutationFn: backupStations });
}
