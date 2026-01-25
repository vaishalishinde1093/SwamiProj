"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/bridge";

export function LoginForm() {
  const router = useRouter();
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await api<{ ok: boolean }>("/api/admin/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ password })
      });
      router.push("/dashboard");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Login failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="space-y-3">
      <label className="space-y-1 block">
        <div className="text-xs text-muted">Password</div>
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          className="w-full rounded-lg bg-panel2/80 border border-black/10 px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-brand/60"
          autoFocus
        />
      </label>

      {error ? <div className="text-sm text-danger">{error}</div> : null}

      <button
        type="submit"
        disabled={busy}
        className="w-full inline-flex items-center justify-center gap-2 rounded-lg bg-brand px-4 py-2 text-sm font-medium text-white hover:bg-brand/90 transition disabled:opacity-60"
      >
        {busy ? "Signing in..." : "Sign in"}
      </button>
    </form>
  );
}

export function LogoutButton() {
  const router = useRouter();
  const [busy, setBusy] = useState(false);

  async function logout() {
    setBusy(true);
    try {
      await api<{ ok: boolean }>("/api/admin/v1/auth/logout", { method: "POST" });
    } finally {
      setBusy(false);
      router.push("/login");
    }
  }

  return (
    <button
      type="button"
      onClick={logout}
      disabled={busy}
      className="inline-flex items-center gap-2 rounded-lg bg-black/5 hover:bg-black/10 border border-black/10 px-3 py-2 text-sm transition disabled:opacity-60"
    >
      {busy ? "Logging out..." : "Logout"}
    </button>
  );
}
