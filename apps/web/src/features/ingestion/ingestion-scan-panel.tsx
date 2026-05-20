"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { activityQueryKey } from "@/features/activity/use-activity-query";
import { Button } from "@/components/ui/button";
import { ApiError, runIngestionScan } from "@/lib/api/client";

export function IngestionScanPanel() {
  const queryClient = useQueryClient();
  const [message, setMessage] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function scan() {
    setPending(true);
    setMessage(null);
    try {
      const data = await runIngestionScan();
      await queryClient.invalidateQueries({ queryKey: activityQueryKey("") });
      const warning = data.errors.length > 0 ? ` ${data.errors.length} station gagal discan.` : "";
      setMessage(`${data.photos.length} stable JPG terdeteksi.${warning}`);
    } catch (error) {
      setMessage(error instanceof ApiError ? `${error.message} ${error.action}` : "Scan ingestion gagal.");
    } finally {
      setPending(false);
    }
  }

  return (
    <section className="restore-panel">
      <p>Ingestion scan mendeteksi stable JPG tanpa routing dulu.</p>
      <Button disabled={pending} onClick={() => void scan()}>
        {pending ? "Scanning..." : "Scan stable JPG"}
      </Button>
      {message ? <p aria-live="polite">{message}</p> : null}
    </section>
  );
}
