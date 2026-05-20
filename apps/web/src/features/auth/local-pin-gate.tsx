"use client";

import { FormEvent, useEffect, useState } from "react";
import { HealthDashboard } from "@/features/health/health-dashboard";
import { ApiError, getApiBaseUrl, getAuthSession, loginWithPin, logout } from "@/lib/api/client";

type AuthState = "checking_session" | "session_error" | "unauthenticated" | "authenticated";

export function LocalPinGate() {
  const [authState, setAuthState] = useState<AuthState>("checking_session");
  const [pin, setPin] = useState("");
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    let active = true;

    checkSession().then((state) => {
      if (active) setAuthState(state);
    });

    return () => {
      active = false;
    };
  }, []);

  async function checkSession(): Promise<AuthState> {
    try {
      const session = await getAuthSession();
      setErrorMessage(null);
      return session.authenticated ? "authenticated" : "unauthenticated";
    } catch {
      setErrorMessage("Tidak bisa mengecek sesi. Pastikan Selfstudio Agent sedang berjalan lalu coba lagi.");
      return "session_error";
    }
  }

  async function handleRetrySessionCheck() {
    setAuthState("checking_session");
    setAuthState(await checkSession());
  }

  async function handleAuthExpired() {
    setAuthState(await checkSession());
  }

  async function handleLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setErrorMessage(null);
    setIsSubmitting(true);

    try {
      const session = await loginWithPin(pin);
      if (!session.authenticated) {
        setErrorMessage("Login tidak berhasil. Cek PIN/password lalu coba lagi.");
        setAuthState("unauthenticated");
        return;
      }
      setAuthState("authenticated");
      setPin("");
    } catch (error) {
      if (error instanceof ApiError) {
        setErrorMessage(`${error.message} ${error.action}`);
      } else {
        setErrorMessage("Login gagal. Pastikan Selfstudio Agent sedang berjalan lalu coba lagi.");
      }
      setAuthState("unauthenticated");
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleLogout() {
    setErrorMessage(null);
    setIsSubmitting(true);

    try {
      await logout();
      setAuthState("unauthenticated");
    } catch {
      setErrorMessage("Logout gagal. Coba refresh dashboard atau restart aplikasi.");
    } finally {
      setIsSubmitting(false);
    }
  }

  if (authState === "checking_session") {
    return (
      <main className="shell">
        <section className="card">
          <p className="eyebrow">Selfstudio Local Admin</p>
          <h1>Mengecek sesi operator</h1>
          <p>Mohon tunggu. Operational dashboard belum ditampilkan sampai sesi terverifikasi.</p>
        </section>
      </main>
    );
  }

  if (authState === "session_error") {
    return (
      <main className="shell">
        <section className="card">
          <p className="eyebrow">Selfstudio Local Admin</p>
          <h1>Sesi belum bisa diverifikasi</h1>
          <p>{errorMessage}</p>
          <button disabled={isSubmitting} onClick={handleRetrySessionCheck} type="button">
            Cek sesi lagi
          </button>
        </section>
      </main>
    );
  }

  if (authState === "unauthenticated") {
    return (
      <main className="shell">
        <section className="card">
          <p className="eyebrow">Selfstudio Local Admin</p>
          <h1>Masuk operator</h1>
          <p>Masukkan PIN/password lokal untuk membuka dashboard operasional event.</p>
          <form className="auth-form" onSubmit={handleLogin}>
            <label htmlFor="operator-pin">PIN/password operator</label>
            <input
              id="operator-pin"
              autoComplete="current-password"
              autoFocus
              disabled={isSubmitting}
              onChange={(event) => setPin(event.target.value)}
              placeholder="Masukkan PIN/password"
              type="password"
              value={pin}
            />
            {errorMessage ? <p className="error-message">{errorMessage}</p> : null}
            <button disabled={isSubmitting || pin.trim() === ""} type="submit">
              {isSubmitting ? "Memproses..." : "Masuk dashboard"}
            </button>
          </form>
          <p>
            Agent API: <code>{getApiBaseUrl()}</code>
          </p>
        </section>
      </main>
    );
  }

  return (
    <HealthDashboard
      authErrorMessage={errorMessage}
      logoutDisabled={isSubmitting}
      onAuthExpired={handleAuthExpired}
      onLogout={handleLogout}
    />
  );
}
