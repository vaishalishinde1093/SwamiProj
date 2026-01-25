"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { api, AdminGroup } from "@/lib/bridge";
import { t, sevaTypeLabel } from "@/lib/strings";
import { RefreshIcon, SaveIcon, SpinnerIcon } from "@/components/devotional-icons";
import { ProgressPopup } from "@/components/progress-popup";

export function GroupsClient() {
  const [groups, setGroups] = useState<AdminGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [toast, setToast] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [expandedSevaType, setExpandedSevaType] = useState<string | null>(null);
  const [progressOpen, setProgressOpen] = useState(false);
  const [progressTitle, setProgressTitle] = useState("");
  const [progressMessage, setProgressMessage] = useState<string | null>(null);
  const [progressStatus, setProgressStatus] = useState<"loading" | "success" | "error">("loading");

  useEffect(() => {
    void load();
  }, []);

  async function load() {
    setToast(null);
    setError(null);
    setLoading(true);
    try {
      const data = await api<{ groups: AdminGroup[]; hash?: string }>("/api/admin/v1/groups");
      setGroups(data.groups);
    } catch (e) {
      setError(e instanceof Error ? e.message : t("groups.failedToLoad"));
    } finally {
      setLoading(false);
    }
  }

  const grouped = useMemo(() => {
    const m = new Map<string, AdminGroup[]>();
    for (const g of groups) {
      m.set(g.seva_type, [...(m.get(g.seva_type) ?? []), g]);
    }
    for (const [k, v] of m) {
      v.sort((a, b) => a.number - b.number);
      m.set(k, v);
    }
    return Array.from(m.entries()).sort(([a], [b]) => a.localeCompare(b));
  }, [groups]);

  function updateLocal(sevaType: string, groupNo: number, patch: Partial<AdminGroup>) {
    setGroups((prev) =>
      prev.map((g) =>
        g.seva_type === sevaType && g.number === groupNo ? { ...g, ...patch } : g
      )
    );
  }

  function toggleExpanded(sevaType: string, groupNo: number) {
    const key = `${sevaType}:${groupNo}`;
    setExpanded((prev) => (prev === key ? null : key));
  }

  function toggleSevaType(sevaType: string) {
    setExpanded(null);
    setExpandedSevaType((prev) => (prev === sevaType ? null : sevaType));
  }

  async function save(g: AdminGroup) {
    const key = `${g.seva_type}:${g.number}`;
    setProgressTitle(t("groups.save"));
    setProgressMessage(`${sevaTypeLabel(g.seva_type)} ${t("groups.group")} ${g.number}`);
    setProgressStatus("loading");
    setProgressOpen(true);
    setBusy(key);
    setToast(null);
    try {
      await api<any>(`/api/admin/v1/groups/${encodeURIComponent(g.seva_type)}/${g.number}`, {
        method: "PUT",
        body: JSON.stringify(g)
      });
      const msg = `${t("groups.savedPrefix")} ${sevaTypeLabel(g.seva_type)} ${t("groups.group")} ${g.number}`;
      setToast(msg);
      setProgressStatus("success");
      setProgressMessage(msg);
    } catch (e) {
      const msg = e instanceof Error ? e.message : t("groups.saveFailed");
      setToast(msg);
      setProgressStatus("error");
      setProgressMessage(msg);
    } finally {
      setBusy(null);
    }
  }

  async function reloadConfig() {
    setProgressTitle(t("groups.reload"));
    setProgressMessage(null);
    setProgressStatus("loading");
    setProgressOpen(true);
    setBusy("reload");
    setToast(null);
    try {
      await api<any>("/api/admin/v1/config/reload", { method: "POST" });
      await load();
      const msg = t("groups.reloaded");
      setToast(msg);
      setProgressStatus("success");
      setProgressMessage(msg);
    } catch (e) {
      const msg = e instanceof Error ? e.message : t("groups.reloadFailed");
      setToast(msg);
      setProgressStatus("error");
      setProgressMessage(msg);
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="space-y-4">
      <ProgressPopup
        open={progressOpen}
        title={progressTitle}
        message={progressMessage}
        status={progressStatus}
        onClose={() => setProgressOpen(false)}
      />
      <div className="rounded-2xl bg-panel/60 border border-black/10 shadow-soft p-5">
        <div className="flex items-start justify-between gap-4">
          <div>
            <div className="text-sm text-muted">{t("groups.kicker")}</div>
            <h1 className="text-xl font-semibold tracking-tight">{t("groups.title")}</h1>
            <p className="mt-1 text-sm text-muted">
              {t("groups.subtitlePrefix")} <span className="text-text">config/groups.yaml</span>.
            </p>
          </div>
          <div className="flex gap-2">
            <button
              onClick={reloadConfig}
              disabled={busy !== null}
              className="inline-flex items-center gap-2 rounded-lg bg-black/5 hover:bg-black/10 border border-black/10 px-3 py-2 text-sm transition disabled:opacity-60"
            >
              {busy === "reload" ? (
                <SpinnerIcon className="h-4 w-4 animate-spin" />
              ) : (
                <RefreshIcon className="h-4 w-4" />
              )}
              {t("groups.reload")}
            </button>
            <button
              onClick={load}
              disabled={busy !== null}
              className="inline-flex items-center gap-2 rounded-lg bg-black/5 hover:bg-black/10 border border-black/10 px-3 py-2 text-sm transition disabled:opacity-60"
            >
              {loading ? <SpinnerIcon className="h-4 w-4 animate-spin" /> : null}
              {t("groups.refresh")}
            </button>
          </div>
        </div>
      </div>

      {toast ? (
        <div className="rounded-xl bg-black/5 border border-black/10 px-4 py-3 text-sm">{toast}</div>
      ) : null}

      {loading ? (
        <div className="text-sm text-muted">{t("groups.loading")}</div>
      ) : error ? (
        <div className="text-sm text-danger">{error}</div>
      ) : (
        <div className="grid grid-cols-1 gap-4">
          {grouped.map(([sevaType, gl]) => (
            <div
              key={sevaType}
              className="rounded-2xl bg-panel/60 border border-black/10 shadow-soft overflow-hidden"
            >
              <button
                type="button"
                onClick={() => toggleSevaType(sevaType)}
                className="w-full text-left px-5 py-4 hover:bg-black/10 transition cursor-pointer"
              >
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <div className="text-xs text-muted">{t("groups.sevaType")}</div>
                    <div className="font-semibold tracking-tight">{sevaTypeLabel(sevaType)}</div>
                    <div className="mt-0.5 text-xs text-muted">{gl.length} {t("groups.group")}</div>
                  </div>
                  <div className="text-sm text-muted font-semibold">{expandedSevaType === sevaType ? "−" : "+"}</div>
                </div>
              </button>

              {expandedSevaType === sevaType ? (
                <div className="p-4 space-y-3 border-t border-black/10 bg-black/5">
                  {gl.map((g) => (
                    <div key={`${g.seva_type}:${g.number}`} className="rounded-xl bg-black/5 border border-black/10">
                      <button
                        type="button"
                        onClick={() => toggleExpanded(g.seva_type, g.number)}
                        className="w-full text-left p-4 hover:bg-black/10 transition cursor-pointer"
                      >
                        <div className="flex items-center justify-between gap-3">
                          <div>
                            <div className="text-sm font-semibold">
                              {t("groups.group")} {g.number}
                            </div>
                            <div className="mt-0.5 text-xs text-muted">
                              {g.name ? g.name : t("groups.fields.name")}
                            </div>
                          </div>
                          <div className="text-xs text-muted">
                            {expanded === `${g.seva_type}:${g.number}` ? "−" : "+"}
                          </div>
                        </div>
                      </button>

                      {expanded === `${g.seva_type}:${g.number}` ? (
                        <div className="px-4 pb-4">
                          <div className="flex flex-col md:flex-row md:items-center justify-between gap-3">
                            <div className="text-xs text-muted">
                              {t("groups.fields.jid")}: <span className="text-text">{g.jid || "—"}</span>
                            </div>
                            <div className="flex gap-2">
                              <Link
                                href={`/groups/${encodeURIComponent(g.seva_type)}/${g.number}/members`}
                                className="inline-flex items-center gap-2 rounded-lg bg-black/5 hover:bg-black/10 border border-black/10 px-3 py-2 text-xs transition"
                              >
                                {t("groups.editMembers")}
                              </Link>
                              <button
                                onClick={() => save(g)}
                                disabled={busy !== null}
                                className="inline-flex items-center gap-2 rounded-lg bg-brand px-3 py-2 text-xs font-medium hover:bg-brand/90 transition disabled:opacity-60"
                              >
                                {busy === `${g.seva_type}:${g.number}` ? (
                                  <SpinnerIcon className="h-4 w-4 animate-spin" />
                                ) : (
                                  <SaveIcon className="h-4 w-4" />
                                )}
                                {t("groups.save")}
                              </button>
                            </div>
                          </div>

                          <div className="mt-3 grid grid-cols-1 md:grid-cols-2 gap-3">
                            <label className="space-y-1">
                              <div className="text-xs text-muted">{t("groups.fields.name")}</div>
                              <input
                                value={g.name}
                                onChange={(e) => updateLocal(g.seva_type, g.number, { name: e.target.value })}
                                className="w-full rounded-lg bg-panel2/80 border border-black/10 px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-brand/60"
                              />
                            </label>
                            <label className="space-y-1">
                              <div className="text-xs text-muted">{t("groups.fields.jid")}</div>
                              <input
                                value={g.jid}
                                onChange={(e) => updateLocal(g.seva_type, g.number, { jid: e.target.value })}
                                className="w-full rounded-lg bg-panel2/80 border border-black/10 px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-brand/60"
                              />
                            </label>
                            <label className="space-y-1">
                              <div className="text-xs text-muted">{t("groups.fields.csvPath")}</div>
                              <input
                                value={g.csv_path}
                                onChange={(e) => updateLocal(g.seva_type, g.number, { csv_path: e.target.value })}
                                className="w-full rounded-lg bg-panel2/80 border border-black/10 px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-brand/60"
                              />
                            </label>
                            <div className="grid grid-cols-2 gap-3">
                              <label className="space-y-1">
                                <div className="text-xs text-muted">{t("groups.fields.maxAdhyas")}</div>
                                <input
                                  type="number"
                                  value={g.max_adhyas}
                                  onChange={(e) =>
                                    updateLocal(g.seva_type, g.number, { max_adhyas: Number(e.target.value) })
                                  }
                                  className="w-full rounded-lg bg-panel2/80 border border-black/10 px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-brand/60"
                                />
                              </label>
                              <label className="space-y-1">
                                <div className="text-xs text-muted">{t("groups.fields.maxPollSize")}</div>
                                <input
                                  type="number"
                                  value={g.max_poll_size}
                                  onChange={(e) =>
                                    updateLocal(g.seva_type, g.number, { max_poll_size: Number(e.target.value) })
                                  }
                                  className="w-full rounded-lg bg-panel2/80 border border-black/10 px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-brand/60"
                                />
                              </label>
                            </div>
                          </div>
                        </div>
                      ) : null}
                    </div>
                  ))}
                </div>
              ) : null}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
