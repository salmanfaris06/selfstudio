"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ApiError, type EligibleSession, type QuarantineItem } from "@/lib/api/client";
import { useAssignQuarantineMutation } from "./use-assign-quarantine-mutation";
import { useEligibleSessionsQuery } from "./use-eligible-sessions-query";
import { useQuarantineQuery } from "./use-quarantine-query";

export function QuarantineReview() {
  const query = useQuarantineQuery("quarantined");
  return (
    <section className="quarantine-review" aria-label="Review quarantine foto">
      <div>
        <p className="eyebrow">Manual Recovery</p>
        <h2>Review quarantined photos</h2>
        <p>Operator bisa melihat metadata aman dan meng-assign foto ke session eligible yang divalidasi oleh Go agent.</p>
      </div>
      {query.isLoading ? <p aria-live="polite">Memuat daftar quarantine...</p> : null}
      {query.isError ? <p className="error-message">{formatError(query.error)}</p> : null}
      {query.data && query.data.items.length === 0 ? <p className="success-message">Tidak ada foto berstatus quarantined yang perlu direview.</p> : null}
      <div className="quarantine-grid">
        {query.data?.items.map((item) => <QuarantineCard item={item} key={item.quarantine_id} />)}
      </div>
    </section>
  );
}

function QuarantineCard({ item }: { item: QuarantineItem }) {
  const [selectedSessionId, setSelectedSessionId] = useState("");
  const eligibleQuery = useEligibleSessionsQuery(item.quarantine_id);
  const assignMutation = useAssignQuarantineMutation();
  const selected = eligibleQuery.data?.sessions.find((session) => session.session_id === selectedSessionId);
  const assign = async () => {
    if (!selected) return;
    const confirmed = window.confirm(`Assign manual quarantine ${item.quarantine_id} ke ${sessionLabel(selected)}? Tindakan ini adalah recovery manual dan akan membuat routed photo trace.`);
    if (!confirmed) return;
    await assignMutation.mutateAsync({ quarantineId: item.quarantine_id, sessionId: selected.session_id });
  };
  return (
    <Card>
      <CardHeader>
        <CardTitle>{item.quarantine_id}</CardTitle>
        <CardDescription>{reasonLabel(item.reason)} • status: {item.status}</CardDescription>
      </CardHeader>
      <CardContent>
        <dl className="station-detail-list">
          <div><dt>Station</dt><dd>{item.station_id}</dd></div>
          <div><dt>Source path</dt><dd>{item.source_path}</dd></div>
          <div><dt>Size</dt><dd>{item.source_size_bytes} bytes</dd></div>
          <div><dt>Detected</dt><dd>{formatDate(item.detected_at)}</dd></div>
          <div><dt>Stable</dt><dd>{formatDate(item.stable_at)}</dd></div>
          <div><dt>Quarantined</dt><dd>{formatDate(item.quarantined_at)}</dd></div>
          <div><dt>Related session</dt><dd>{item.related_session_id || "-"}</dd></div>
        </dl>
        {eligibleQuery.isError ? <p className="error-message">{formatError(eligibleQuery.error)}</p> : null}
        <label className="field-label" htmlFor={`session-${item.quarantine_id}`}>Eligible target session</label>
        <select id={`session-${item.quarantine_id}`} value={selectedSessionId} onChange={(event) => setSelectedSessionId(event.target.value)}>
          <option value="">Pilih session eligible</option>
          {eligibleQuery.data?.sessions.map((session) => <option key={session.session_id} value={session.session_id}>{sessionLabel(session)}</option>)}
        </select>
        {selected ? <p className="helper-text">Konfirmasi wajib: {selected.eligibility_reason}. Session {selected.status}; customer {selected.customer_name}; order {selected.order_number}.</p> : null}
        {assignMutation.isError ? <p className="error-message">{formatError(assignMutation.error)}</p> : null}
        {assignMutation.isSuccess ? <p className="success-message">Assignment berhasil. Daftar quarantine, session, station summary, dan activity log akan direfresh.</p> : null}
        <Button disabled={!selected || assignMutation.isPending} onClick={() => void assign()} type="button">{assignMutation.isPending ? "Meng-assign..." : "Confirm manual assignment"}</Button>
      </CardContent>
    </Card>
  );
}

function sessionLabel(session: EligibleSession) {
  return `${session.session_id} • ${session.customer_name} • ${session.order_number} • ${session.station_id}`;
}

function reasonLabel(reason: QuarantineItem["reason"]) {
  if (reason === "late_photo") return "late_photo: foto datang setelah session lock";
  return "no_active_session: tidak ada session aktif";
}

function formatDate(value: string) {
  return new Date(value).toLocaleString("id-ID");
}

function formatError(error: unknown) {
  if (error instanceof ApiError) return `${error.message} ${error.action}`;
  if (error instanceof Error) return error.message;
  return "Aksi gagal. Refresh dashboard lalu coba lagi.";
}
