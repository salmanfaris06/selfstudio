"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ApiError, recordConfigPlaceholderAction } from "@/lib/api/client";
import { useActivityQuery } from "./use-activity-query";

const actionOptions = [
  { value: "", label: "Semua aksi" },
  { value: "login.success", label: "Login berhasil" },
  { value: "login.failure", label: "Login gagal" },
  { value: "logout.success", label: "Logout" },
  { value: "health.recheck", label: "Health recheck" },
  { value: "config.placeholder", label: "Config placeholder" },
];

export function ActivityLog() {
  const [actionType, setActionType] = useState("");
  const [configMessage, setConfigMessage] = useState<string | null>(null);
  const [isRecordingConfig, setIsRecordingConfig] = useState(false);
  const activityQuery = useActivityQuery(actionType);

  async function handleConfigPlaceholder() {
    if (isRecordingConfig) return;
    setIsRecordingConfig(true);
    setConfigMessage(null);
    try {
      await recordConfigPlaceholderAction();
      setConfigMessage("Config placeholder action berhasil dicatat.");
      await activityQuery.refetch();
    } catch (error) {
      if (error instanceof ApiError) {
        setConfigMessage(`${error.message} ${error.action}`);
      } else {
        setConfigMessage("Config placeholder action gagal dicatat.");
      }
    } finally {
      setIsRecordingConfig(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <div className="status-card-heading">
          <div>
            <CardTitle>Activity log</CardTitle>
            <CardDescription>Riwayat aksi operator terbaru dengan pesan aman tanpa data sensitif.</CardDescription>
          </div>
          <Button disabled={isRecordingConfig} onClick={handleConfigPlaceholder}>
            {isRecordingConfig ? "Mencatat..." : "Catat config placeholder"}
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <div className="activity-toolbar">
          <label htmlFor="activity-action-filter">Filter aksi</label>
          <select
            id="activity-action-filter"
            onChange={(event) => setActionType(event.target.value)}
            value={actionType}
          >
            {actionOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
          <Button disabled={activityQuery.isFetching} onClick={() => void activityQuery.refetch()}>
            Refresh log
          </Button>
        </div>

        {configMessage ? <p className="activity-message">{configMessage}</p> : null}

        {activityQuery.isLoading ? <p>Memuat activity log...</p> : null}
        {activityQuery.isError ? (
          <div className="activity-error" role="alert">
            <p>{formatActivityError(activityQuery.error)}</p>
            <Button onClick={() => void activityQuery.refetch()}>Coba lagi</Button>
          </div>
        ) : null}

        {activityQuery.data ? (
          <div className="activity-list">
            {activityQuery.data.entries.length === 0 ? <p>Belum ada activity log untuk filter ini.</p> : null}
            {activityQuery.data.entries.map((entry) => (
              <article className="activity-entry" key={entry.id}>
                <div>
                  <time dateTime={entry.occurred_at}>{formatTimestamp(entry.occurred_at)}</time>
                  <h3>{entry.action_type}</h3>
                  <p>{entry.message}</p>
                </div>
                <Badge>{entry.result}</Badge>
              </article>
            ))}
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}

function formatActivityError(error: Error) {
  if (error instanceof ApiError) {
    return `${error.message} ${error.action}`;
  }
  return "Activity log gagal dimuat. Pastikan Go agent berjalan lalu coba lagi.";
}

function formatTimestamp(value: string) {
  const parsed = Date.parse(value);
  if (Number.isNaN(parsed)) {
    return "Waktu tidak valid";
  }

  return new Intl.DateTimeFormat("id-ID", {
    dateStyle: "medium",
    timeStyle: "medium",
  }).format(new Date(parsed));
}
