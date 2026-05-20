import { useMutation } from "@tanstack/react-query";
import { discoverGPhoto2Cameras } from "@/lib/api/client";

export function useGPhoto2DiscoveryMutation() {
  return useMutation({ mutationFn: discoverGPhoto2Cameras });
}
