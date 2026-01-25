"use client";

import { useEffect } from "react";
import { SpinnerIcon } from "@/components/devotional-icons";

export type ProgressPopupStatus = "loading" | "success" | "error";

export function ProgressPopup({
  open,
  title,
  message,
  status,
  onClose,
  autoCloseMs
}: {
  open: boolean;
  title: string;
  message?: string | null;
  status: ProgressPopupStatus;
  onClose: () => void;
  autoCloseMs?: number;
}) {
  useEffect(() => {
    if (!open) return;
    if (status === "loading") return;
    if (autoCloseMs === undefined) return;

    const ms = autoCloseMs;
    const t = window.setTimeout(() => onClose(), ms);
    return () => window.clearTimeout(t);
  }, [open, status, autoCloseMs, onClose]);

  if (!open) return null;

  const badgeClass =
    status === "loading"
      ? "bg-black/5 border-black/10 text-muted"
      : status === "success"
        ? "bg-green-500/10 border-green-500/20 text-green-800"
        : "bg-red-500/10 border-red-500/20 text-red-800";

  const badgeText = status === "loading" ? "Working" : status === "success" ? "Done" : "Error";

  return (
    <div className="fixed inset-0 z-50">
      <div className="absolute inset-0 bg-black/40 backdrop-blur-sm" />
      <div className="absolute inset-0 flex items-center justify-center p-4">
        <div className="w-full max-w-md rounded-2xl bg-panel/90 border border-black/10 shadow-soft overflow-hidden">
          <div className="px-5 py-4 border-b border-black/10 flex items-center justify-between gap-3">
            <div className="min-w-0">
              <div className="text-sm text-muted">{title}</div>
              {message ? <div className="mt-1 text-sm font-medium text-text break-words">{message}</div> : null}
            </div>
            <div
              className={`shrink-0 inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs border ${badgeClass}`}
            >
              {status === "loading" ? <SpinnerIcon className="h-4 w-4 animate-spin" /> : null}
              {badgeText}
            </div>
          </div>

          <div className="px-5 py-4">
            <button
              type="button"
              onClick={onClose}
              disabled={status === "loading"}
              className="w-full inline-flex items-center justify-center rounded-lg bg-black/5 hover:bg-black/10 border border-black/10 px-3 py-2 text-sm transition disabled:opacity-60"
            >
              OK
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
