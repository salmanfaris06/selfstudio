"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ApiError, type ProcessingQueueItem, type ProcessingQueueSummary } from "@/lib/api/client";
import { useProcessingQueueQuery } from "./use-processing-queue-query";
import { useRetryProcessingMutation } from "./use-retry-processing-mutation";

const bucketLabels: Array<{ key: keyof Pick<ProcessingQueueSummary, "pending" | "processing" | "processed" | "failed" | "retrying">; label: string; help: string }> = [
  { key: "pending", label: "Pending", help: "Menunggu graded processing" },
  { key: "processing", label: "Processing", help: "Sedang dibuat graded JPG" },
  { key: "processed", label: "Processed", help: "Selesai / succeeded" },
  { key: "failed", label: "Failed", help: "Butuh perhatian operator" },
  { key: "retrying", label: "Retrying", help: "Disiapkan untuk retry policy berikutnya" },
];

export function ProcessingQueuePanel({ eventStreamState }: { eventStreamState: "connecting" | "open" | "error" | "closed" }) {
  const query = useProcessingQueueQuery({ limit: 20 });
  const retryMutation = useRetryProcessingMutation();

  return (
    <section aria-label="Processing queue dan status foto" className="activity-section">
      <Card>
        <CardHeader>
          <CardTitle>Processing queue</CardTitle>
          <CardDescription>
            Status per-foto dari Go agent. Dashboard tetap bisa dipakai walau processing lambat atau event stream sedang bermasalah.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <p aria-live="polite">Event stream: {eventStreamCopy(eventStreamState)}</p>
          {query.isLoading ? <p>Memuat queue processing dari Go agent...</p> : null}
          {query.isError ? (
            <div role="alert">
              <p>{formatError(query.error)}</p>
              <Button disabled={query.isFetching} onClick={() => void query.refetch()} type="button">
                Coba muat queue lagi
              </Button>
            </div>
          ) : null}
          {query.data ? (
            <>
              <QueueSummaryView summary={query.data.summary} />
              {query.data.items.length === 0 ? (
                <p>Belum ada foto di processing queue. Saat foto routed dan original tersimpan, status akan muncul di sini.</p>
              ) : (
                <div className="queue-list" aria-label="Daftar status processing foto terbaru">
                  {query.data.items.map((item) => (
                    <QueueItemRow item={item} key={item.photo_id} onRetry={(photoId) => retryMutation.mutate(photoId)} retryPending={retryMutation.isPending && retryMutation.variables === item.photo_id} retryError={retryMutation.variables === item.photo_id && retryMutation.error ? retryMutation.error : null} />
                  ))}
                </div>
              )}
            </>
          ) : null}
        </CardContent>
      </Card>
    </section>
  );
}

function QueueSummaryView({ summary }: { summary: ProcessingQueueSummary }) {
  return (
    <div className="health-grid" aria-label="Ringkasan status processing">
      <div>
        <strong>Total</strong>
        <p>{summary.total} foto tracked</p>
        <small>Not eligible: {summary.not_eligible}</small>
      </div>
      {bucketLabels.map((bucket) => (
        <div key={bucket.key}>
          <strong>{bucket.label}</strong>
          <p>{summary[bucket.key]} foto</p>
          <small>{bucket.help}</small>
        </div>
      ))}
      <div>
        <strong>Current job</strong>
        <p>{summary.current_job ? `${summary.current_job.photo_id} (${summary.current_job.station_id})` : "Tidak ada job berjalan"}</p>
        <small>Update terakhir: {summary.last_updated_at ? formatDate(summary.last_updated_at) : "belum ada"}</small>
      </div>
    </div>
  );
}

function QueueItemRow({ item, onRetry, retryPending, retryError }: { item: ProcessingQueueItem; onRetry: (photoId: string) => void; retryPending: boolean; retryError: Error | null }) {
  return (
    <article className="card subtle-card">
      <h3>{item.photo_id}</h3>
      <p>
        Station {item.station_id} · Session {item.session_id}
      </p>
      <p>
        Status: {statusLabel(item.graded_processing_status)} · Original: {item.original_save_status} · Eligibility: {item.processing_status}
      </p>
      <p>Attempt / retry count: {item.graded_attempt_count}</p>
      {item.graded_last_error ? <p>Failure reason: {item.graded_last_error}</p> : <p>Failure reason: tidak ada</p>}
      {item.graded_last_error ? <p>{failureActionCopy(item.graded_last_error)}</p> : null}
      {item.graded_processing_status === "failed" ? (
        <div>
          <Button disabled={retryPending} onClick={() => onRetry(item.photo_id)} type="button">
            {retryPending ? "Retry sedang dimulai..." : "Retry processing"}
          </Button>
          {retryError ? <p role="alert">{formatError(retryError)}</p> : null}
        </div>
      ) : null}
      <p>Update terakhir: {formatDate(item.last_updated_at)}</p>
    </article>
  );
}

function statusLabel(status: ProcessingQueueItem["graded_processing_status"]) {
  if (status === "processed") return "processed / succeeded";
  if (status === "not_eligible") return "not eligible";
  return status;
}

function failureActionCopy(reason: string) {
  if (reason.includes("LUT_PROCESSOR_UNAVAILABLE")) return "Action: install ImageMagick 7 (`magick`) lalu klik Retry processing.";
  if (reason.includes("LUT_MISSING") || reason.includes("LUT_INVALID") || reason.includes("LUT_UNREADABLE")) return "Action: perbaiki file LUT station/session lalu klik Retry processing.";
  return "Action: retry nanti setelah penyebab gagal diperbaiki; original JPG tetap aman.";
}

function eventStreamCopy(state: "connecting" | "open" | "error" | "closed") {
  if (state === "open") return "terhubung, queue akan refresh via SSE.";
  if (state === "connecting") return "sedang menghubungkan, data tetap refresh berkala.";
  if (state === "error") return "belum stabil, gunakan tombol refresh bila perlu.";
  return "ditutup, buka ulang dashboard untuk real-time update.";
}

function formatDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("id-ID");
}

function formatError(error: Error) {
  if (error instanceof ApiError) return `${error.message} ${error.action}`;
  return "Queue processing belum bisa dimuat. Pastikan Go agent berjalan lalu coba lagi.";
}
