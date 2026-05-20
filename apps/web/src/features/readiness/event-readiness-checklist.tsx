"use client";

import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ApiError, type EventReadinessItem } from "@/lib/api/client";
import { useEventReadinessQuery } from "./use-event-readiness-query";
import { useRunEventReadinessCheckMutation } from "./use-run-event-readiness-check-mutation";

export function EventReadinessChecklist() {
  const readinessQuery = useEventReadinessQuery();
  const mutation = useRunEventReadinessCheckMutation();
  const [message, setMessage] = useState<{ kind: "success" | "error"; text: string } | null>(null);

  async function handleCheck() {
    setMessage(null);
    try {
      await mutation.mutateAsync();
      setMessage({ kind: "success", text: "Event readiness checklist selesai diperiksa." });
    } catch (error) {
      setMessage({ kind: "error", text: formatError(error) });
    }
  }

  const readiness = readinessQuery.data?.readiness;
  const grouped = groupItems(readiness?.items ?? []);

  return (
    <section className="event-readiness" aria-label="Event readiness checklist">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Event Preflight</p>
          <h2>Event readiness checklist</h2>
          <p>Satu ringkasan station, storage, cloud, dan operator sebelum session control tersedia.</p>
        </div>
        <Button disabled={mutation.isPending} onClick={() => void handleCheck()}>
          {mutation.isPending ? "Memeriksa..." : "Run event readiness check"}
        </Button>
      </div>

      {readinessQuery.isLoading ? <p>Memuat event readiness...</p> : null}
      {readinessQuery.isError ? <p className="error-message">{formatError(readinessQuery.error)}</p> : null}
      {message ? <p className={message.kind === "success" ? "success-message" : "error-message"}>{message.text}</p> : null}

      {readiness ? (
        <Card>
          <CardHeader>
            <div className="status-card-heading">
              <CardTitle>{readiness.label}</CardTitle>
              <Badge>{readiness.status}</Badge>
            </div>
            <CardDescription>{readiness.action}</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="readiness-panel">
              <p>
                <strong>Session start:</strong> {readiness.session_start_available ? "available" : "unavailable"} — {readiness.session_start_action}
              </p>
              <Button disabled type="button">Start session belum tersedia</Button>
              {Object.entries(grouped).map(([category, items]) => (
                <div key={category}>
                  <h3>{category}</h3>
                  <ul>
                    {items.map((item) => (
                      <li key={`${item.category}:${item.item_key}`}>
                        <strong>{item.item_key}</strong>: {item.status} — {item.label}. {item.action}
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      ) : null}
    </section>
  );
}

function groupItems(items: EventReadinessItem[]) {
  return items.reduce<Record<string, EventReadinessItem[]>>((groups, item) => {
    groups[item.category] ??= [];
    groups[item.category].push(item);
    return groups;
  }, {});
}

function formatError(error: unknown): string {
  if (error instanceof ApiError) return `${error.message} ${error.action}`;
  return "Event readiness gagal diproses. Coba lagi atau restart aplikasi.";
}
