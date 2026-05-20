"use client";

import { useEffect, useState } from "react";
import {
  ApiError,
  checkCloudConnection,
  getCloudSettings,
  previewCloudFolderPath,
  updateCloudSettings,
  type CloudSettings,
} from "@/lib/api/client";

const labels: Record<string, string> = {
  not_configured: "NOT CONFIGURED",
  authorized: "AUTHORIZED",
  failed: "FAILED",
  checking: "CHECKING",
};

export function CloudSettingsPanel() {
  const [settings, setSettings] = useState<CloudSettings | null>(null);
  const [rootFolderId, setRootFolderId] = useState("");
  const [rootFolderName, setRootFolderName] = useState("");
  const [credential, setCredential] = useState("");
  const [preview, setPreview] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    void load();
  }, []);
  useEffect(() => {
    void refreshPreview();
  }, []);

  async function load() {
    try {
      const data = await getCloudSettings();
      setSettings(data);
      setRootFolderId(data.drive_root_folder_id ?? "");
      setRootFolderName(data.drive_root_folder_name ?? "");
    } catch (error) {
      setMessage(
        error instanceof Error
          ? error.message
          : "Google Drive settings tidak bisa dimuat.",
      );
    }
  }

  async function refreshPreview() {
    try {
      const data = await previewCloudFolderPath({
        customer_name: "Customer Demo",
        order_number: "SO 001",
        station_id: "station-1",
        session_id: "sess-demo",
      });
      setPreview(data.folder_path);
    } catch {
      setPreview("Preview folder belum valid");
    }
  }

  async function save() {
    setBusy(true);
    setMessage("");
    try {
      const data = await updateCloudSettings({
        provider: "google_drive",
        drive_root_folder_id: rootFolderId,
        drive_root_folder_name: rootFolderName || undefined,
        service_account_json: credential || undefined,
      });
      setSettings(data);
      setCredential("");
      setMessage(
        "Google Drive settings tersimpan. Credential disimpan server-side dan tidak ditampilkan ulang.",
      );
    } catch (error) {
      setMessage(
        error instanceof ApiError
          ? `${error.message} (${error.action})`
          : "Google Drive settings gagal disimpan.",
      );
    } finally {
      setBusy(false);
    }
  }

  async function check() {
    setBusy(true);
    setMessage("");
    try {
      setSettings(await checkCloudConnection());
      setMessage("Google Drive connection check authorized.");
    } catch (error) {
      setMessage(
        error instanceof ApiError
          ? `${error.message} (${error.action})`
          : "Connection check Google Drive gagal.",
      );
      await load();
    } finally {
      setBusy(false);
    }
  }

  const status = settings?.connection_status ?? "not_configured";
  return (
    <section className="rounded-lg border border-slate-700 bg-slate-900/60 p-4 text-sm text-slate-100">
      <div className="mb-3 flex items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold">Google Drive Settings</h2>
          <p className="text-slate-400">
            Drive delivery config terpisah dari local save/processing. Story ini
            belum menjalankan upload file.
          </p>
        </div>
        <span className="rounded bg-slate-800 px-2 py-1 font-mono">
          {labels[status] ?? status}
        </span>
      </div>
      <div className="grid gap-3 md:grid-cols-2">
        <label>
          Drive root folder ID
          <input
            className="mt-1 w-full rounded bg-slate-950 p-2"
            value={rootFolderId}
            onChange={(event) => setRootFolderId(event.target.value)}
            placeholder="Google Drive folder ID"
          />
        </label>
        <label>
          Root folder name (display only)
          <input
            className="mt-1 w-full rounded bg-slate-950 p-2"
            value={rootFolderName}
            onChange={(event) => setRootFolderName(event.target.value)}
            placeholder="Selfstudio Delivery"
          />
        </label>
      </div>
      <label className="mt-3 block">
        Service account JSON (write-only)
        <textarea
          className="mt-1 h-24 w-full rounded bg-slate-950 p-2 font-mono"
          value={credential}
          onChange={(event) => setCredential(event.target.value)}
          placeholder={
            settings?.credentials_configured
              ? "Credential sudah tersimpan. Isi hanya jika ingin mengganti."
              : "Tempel service account JSON di sini. Jangan bagikan di chat."
          }
        />
      </label>
      <p className="mt-2 text-xs text-amber-200">
        Credential hanya dikirim ke Go agent dan tidak dikembalikan lewat API,
        SSE, activity log, atau UI.
      </p>
      <div className="mt-3 rounded bg-slate-950 p-3">
        <p className="font-semibold">Drive folder naming convention</p>
        <code className="break-all text-xs text-slate-300">
          {settings?.folder_naming_template ??
            "{yyyy}/{mm}/{dd}/{safe_customer_name}/{safe_order_number}/{station_id}/{session_id}"}
        </code>
        <p className="mt-2 break-all text-slate-300">Preview: {preview}</p>
      </div>
      {settings?.last_error_code || settings?.last_error_action ? (
        <p className="mt-3 text-red-300">
          {settings.last_error_code ?? "DRIVE_CHECK_FAILED"}
          {settings.last_error_action ? ` (${settings.last_error_action})` : ""}
        </p>
      ) : null}
      {message ? <p className="mt-3 text-slate-300">{message}</p> : null}
      <div className="mt-4 flex gap-2">
        <button
          className="rounded bg-blue-600 px-3 py-2 disabled:opacity-50"
          disabled={busy}
          onClick={save}
          type="button"
        >
          Save Google Drive Settings
        </button>
        <button
          className="rounded bg-slate-700 px-3 py-2 disabled:opacity-50"
          disabled={busy}
          onClick={check}
          type="button"
        >
          Check Drive Connection
        </button>
      </div>
    </section>
  );
}
