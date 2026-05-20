import { useQuery } from "@tanstack/react-query";
import { getQuarantineItems } from "@/lib/api/client";

export const quarantineQueryKey = ["quarantine"] as const;

export function useQuarantineQuery(status = "quarantined") {
  return useQuery({
    queryKey: [...quarantineQueryKey, { status }],
    queryFn: () => getQuarantineItems({ status }),
    refetchInterval: 15_000,
  });
}
