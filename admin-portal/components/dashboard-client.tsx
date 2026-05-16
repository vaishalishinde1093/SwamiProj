"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { api, AdminGroup, ApiError } from "@/lib/bridge";
import { t, sevaTypeLabel } from "@/lib/strings";
import { DeepakIcon, GhantaIcon, MandirIcon, SpinnerIcon, UsersIcon } from "@/components/devotional-icons";
import { ProgressPopup } from "@/components/progress-popup";

type SevaAction =
  | { kind: "send"; sevaType: string; groupNo: number }
  | { kind: "remind"; sevaType: string; groupNo: number }
  | { kind: "announce"; sevaType: string; groupNo: number };

type BulkSevaAction =
  | { kind: "send"; sevaType: string }
  | { kind: "remind"; sevaType: string }
  | { kind: "announce"; sevaType: string };

function endpointFor(action: SevaAction): string {
  const st = action.sevaType;
  if (action.kind === "send") {
    if (st === "ekadashi_bhagavat") return "/api/v2/ekadashi-bhagavat-seva";
    if (st === "durga_paath") return "/api/v2/durga-paath";
    if (st === "saptahik_swami") return "/api/v2/saptahik-swami-seva";
    if (st === "malhari") return "/api/v2/malhari";
    if (st === "darbar") return "/api/v2/darbar";
    if (st === "chaitra_navratri") return "/api/v2/chaitra-navratri";
  }
  if (action.kind === "remind") {
    if (st === "ekadashi_bhagavat") return "/api/v2/ekadashi-bhagavat-seva/send-reminders";
    if (st === "durga_paath") return "/api/v2/durga-paath/send-reminders";
    if (st === "saptahik_swami") return "/api/v2/saptahik-swami-seva/send-reminders";
    if (st === "malhari") return "/api/v2/malhari/send-reminders";
    if (st === "darbar") return "/api/v2/darbar/send-reminders";
    if (st === "chaitra_navratri") return "/api/v2/chaitra-navratri/send-reminders";
  }
  if (action.kind === "announce") {
    if (st === "ekadashi_bhagavat") return "/api/v2/ekadashi-bhagavat-seva/group-announcement";
    if (st === "durga_paath") return "/api/v2/durga-paath/group-announcement";
    if (st === "saptahik_swami") return "/api/v2/saptahik-swami-seva/group-announcement";
    if (st === "malhari") return "/api/v2/malhari/group-announcement";
    if (st === "darbar") return "/api/v2/darbar/group-announcement";
    if (st === "chaitra_navratri") return "/api/v2/chaitra-navratri/group-announcement";
  }
  return "";
}

export function DashboardClient() {
  const router = useRouter();
  const [groups, setGroups] = useState<AdminGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [toast, setToast] = useState<string | null>(null);
  const [expandedSevaType, setExpandedSevaType] = useState<string | null>(null);
  const [bulkBusy, setBulkBusy] = useState<string | null>(null);
  const [progressOpen, setProgressOpen] = useState(false);
  const [progressTitle, setProgressTitle] = useState("");
  const [progressMessage, setProgressMessage] = useState<string | null>(null);
  const [progressStatus, setProgressStatus] = useState<"loading" | "success" | "error">("loading");

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        setLoading(true);
        const data = await api<{ groups: AdminGroup[] }>("/api/admin/v1/groups");
        if (!cancelled) setGroups(data.groups);
      } catch (e) {
        if (e instanceof ApiError && e.status === 401) {
          router.push("/login");
          return;
        }
        if (!cancelled) setError(e instanceof Error ? e.message : "Failed to load");
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const grouped = useMemo(() => {
    const m = new Map<string, AdminGroup[]>();
    for (const g of groups) {
      const k = g.seva_type;
      m.set(k, [...(m.get(k) ?? []), g]);
    }
    for (const [k, v] of m) {
      v.sort((a, b) => a.number - b.number);
      m.set(k, v);
    }
    return Array.from(m.entries()).sort(([a], [b]) => a.localeCompare(b));
  }, [groups]);

  async function run(action: SevaAction) {
    const ep = endpointFor(action);
    if (!ep) {
      setToast(t("dashboard.noEndpoint"));
      return;
    }

    const kindLabel =
      action.kind === "send" ? t("dashboard.pollMessage") : action.kind === "remind" ? t("dashboard.remind") : t("dashboard.announce");
    setProgressTitle(kindLabel);
    setProgressMessage(`${sevaTypeLabel(action.sevaType)} · ${t("dashboard.group")} ${action.groupNo}`);
    setProgressStatus("loading");
    setProgressOpen(true);

    const key = `${action.kind}:${action.sevaType}:${action.groupNo}`;
    setBusy(key);
    setToast(null);

    try {
      const body = { group_no: action.groupNo };
      const resp = await api<unknown>(ep, { method: "POST", body: JSON.stringify(body) });

      const message =
        resp && typeof resp === "object" && "message" in resp ? String((resp as { message?: unknown }).message ?? "") : "";

      setToast(message ? message : t("dashboard.actionDone"));
      setProgressStatus("success");
      setProgressMessage(message ? message : t("dashboard.actionDone"));
    } catch (e) {
      setToast(e instanceof Error ? e.message : t("dashboard.actionFailed"));
      setProgressStatus("error");
      setProgressMessage(e instanceof Error ? e.message : t("dashboard.actionFailed"));
    } finally {
      setBusy(null);
    }
  }

  async function runBulk(action: BulkSevaAction, groupNumbers: number[]) {
    const kindLabel =
      action.kind === "send" ? t("dashboard.pollMessage") : action.kind === "remind" ? t("dashboard.remind") : t("dashboard.announce");
    setProgressTitle(`${kindLabel} - All Groups`);
    setProgressMessage(`${sevaTypeLabel(action.sevaType)} · ${groupNumbers.length} groups`);
    setProgressStatus("loading");
    setProgressOpen(true);

    const bulkKey = `bulk:${action.kind}:${action.sevaType}`;
    setBulkBusy(bulkKey);
    setToast(null);

    let successCount = 0;
    let failCount = 0;

    for (const groupNo of groupNumbers) {
      const ep = endpointFor({ ...action, groupNo });
      if (!ep || groupNo > 90) continue;

      try {
        const body = { group_no: groupNo };
        await api<unknown>(ep, { method: "POST", body: JSON.stringify(body) });
        successCount++;
      } catch {
        failCount++;
      }
    }

    setBulkBusy(null);
    const resultMsg = `Completed: ${successCount} succeeded, ${failCount} failed`;
    setToast(resultMsg);
    setProgressStatus(failCount === 0 ? "success" : "error");
    setProgressMessage(resultMsg);
  }

  function toggleSevaType(sevaType: string) {
    setExpandedSevaType((prev) => (prev === sevaType ? null : sevaType));
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
            <div className="text-sm text-muted">{t("dashboard.kicker")}</div>
            <h1 className="text-xl font-semibold tracking-tight">{t("dashboard.title")}</h1>
            <p className="mt-1 text-sm text-muted">{t("dashboard.subtitle")}</p>
          </div>
          <Link
            href="/groups"
            className="inline-flex items-center gap-2 rounded-lg bg-black/5 hover:bg-black/10 border border-black/10 px-3 py-2 text-sm transition"
          >
            {t("dashboard.manageGroups")} <UsersIcon className="h-4 w-4" />
          </Link>
        </div>
      </div>

      {toast ? (
        <div className="rounded-xl bg-black/5 border border-black/10 px-4 py-3 text-sm">{toast}</div>
      ) : null}

      {loading ? (
        <div className="text-sm text-muted">{t("dashboard.loadingGroups")}</div>
      ) : error ? (
        <div className="text-sm text-danger">{error}</div>
      ) : (
        <div className="grid grid-cols-1 gap-4">
          {grouped.map(([sevaType, gl]) => (
            <div
              key={sevaType}
              className="rounded-2xl bg-panel/60 border border-black/10 shadow-soft overflow-hidden"
            >
              <div className="px-5 py-4 space-y-3">
                <button
                  type="button"
                  onClick={() => toggleSevaType(sevaType)}
                  className="w-full text-left hover:bg-black/10 transition cursor-pointer rounded-lg p-2 -m-2"
                >
                  <div className="flex items-center justify-between gap-3">
                    <div>
                      <div className="text-xs text-muted">{t("dashboard.sevaType")}</div>
                      <div className="font-semibold tracking-tight">{sevaTypeLabel(sevaType)}</div>
                      <div className="mt-0.5 text-xs text-muted">
                        {gl.length} {t("dashboard.group")}
                      </div>
                    </div>
                    <div className="text-sm text-muted font-semibold">{expandedSevaType === sevaType ? "−" : "+"}</div>
                  </div>
                </button>

                <div className="flex flex-wrap gap-2">
                  <button
                    type="button"
                    onClick={() => runBulk({ kind: "send", sevaType }, gl.map((g) => g.number))}
                    disabled={busy !== null || bulkBusy !== null}
                    className="inline-flex items-center gap-2 rounded-lg bg-brand px-3 py-2 text-xs font-semibold hover:bg-brand/90 transition disabled:opacity-60"
                  >
                    {bulkBusy === `bulk:send:${sevaType}` ? (
                      <SpinnerIcon className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <DeepakIcon className="h-3.5 w-3.5" />
                    )}
                    {t("dashboard.sendSevaToAll")}
                  </button>
                  <button
                    type="button"
                    onClick={() => runBulk({ kind: "remind", sevaType }, gl.map((g) => g.number))}
                    disabled={busy !== null || bulkBusy !== null}
                    className="inline-flex items-center gap-2 rounded-lg bg-black/5 px-3 py-2 text-xs font-semibold hover:bg-black/10 border border-black/10 transition disabled:opacity-60"
                  >
                    {bulkBusy === `bulk:remind:${sevaType}` ? (
                      <SpinnerIcon className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <GhantaIcon className="h-3.5 w-3.5" />
                    )}
                    {t("dashboard.remindAll")}
                  </button>
                  <button
                    type="button"
                    onClick={() => runBulk({ kind: "announce", sevaType }, gl.map((g) => g.number))}
                    disabled={busy !== null || bulkBusy !== null}
                    className="inline-flex items-center gap-2 rounded-lg bg-black/5 px-3 py-2 text-xs font-semibold hover:bg-black/10 border border-black/10 transition disabled:opacity-60"
                  >
                    {bulkBusy === `bulk:announce:${sevaType}` ? (
                      <SpinnerIcon className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <MandirIcon className="h-3.5 w-3.5" />
                    )}
                    {t("dashboard.announceToAll")}
                  </button>
                </div>
              </div>

              {expandedSevaType === sevaType ? (
                <div className="p-2 border-t border-black/10 bg-black/5">
                  {gl.map((g) => (
                    <div
                      key={`${g.seva_type}:${g.number}`}
                      className="flex flex-col md:flex-row md:items-center justify-between gap-3 rounded-xl bg-black/5 hover:bg-black/10 border border-black/10 p-4 m-2 transition"
                    >
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <div className="text-sm font-semibold">
                            {t("dashboard.group")} {g.number}
                          </div>
                          <div className="text-xs text-muted truncate">{g.name}</div>
                        </div>
                        <div className="mt-1 text-xs text-muted truncate">
                          {t("dashboard.jid")}: {g.jid}
                        </div>
                      </div>

                      <div className="flex flex-wrap gap-2">
                        <button
                          onClick={() => run({ kind: "send", sevaType: g.seva_type, groupNo: g.number })}
                          disabled={busy !== null || bulkBusy !== null}
                          className="inline-flex items-center gap-2 rounded-lg bg-brand px-4 py-3 text-sm font-semibold hover:bg-brand/90 transition disabled:opacity-60"
                        >
                          {busy === `send:${g.seva_type}:${g.number}` ? (
                            <SpinnerIcon className="h-4 w-4 animate-spin" />
                          ) : (
                            <DeepakIcon className="h-4 w-4" />
                          )}
                          {t("dashboard.pollMessage")}
                        </button>
                        <button
                          onClick={() => run({ kind: "remind", sevaType: g.seva_type, groupNo: g.number })}
                          disabled={busy !== null || bulkBusy !== null}
                          className="inline-flex items-center gap-2 rounded-lg bg-black/5 px-4 py-3 text-sm font-semibold hover:bg-black/10 border border-black/10 transition disabled:opacity-60"
                        >
                          {busy === `remind:${g.seva_type}:${g.number}` ? (
                            <SpinnerIcon className="h-4 w-4 animate-spin" />
                          ) : (
                            <GhantaIcon className="h-4 w-4" />
                          )}
                          {t("dashboard.remind")}
                        </button>
                        <button
                          onClick={() => run({ kind: "announce", sevaType: g.seva_type, groupNo: g.number })}
                          disabled={busy !== null || bulkBusy !== null}
                          className="inline-flex items-center gap-2 rounded-lg bg-black/5 px-4 py-3 text-sm font-semibold hover:bg-black/10 border border-black/10 transition disabled:opacity-60"
                        >
                          {busy === `announce:${g.seva_type}:${g.number}` ? (
                            <SpinnerIcon className="h-4 w-4 animate-spin" />
                          ) : (
                            <MandirIcon className="h-4 w-4" />
                          )}
                          {t("dashboard.announce")}
                        </button>
                        <Link
                          href={`/groups/${encodeURIComponent(g.seva_type)}/${g.number}/members`}
                          className="inline-flex items-center gap-2 rounded-lg bg-black/5 px-4 py-3 text-sm font-semibold hover:bg-black/10 border border-black/10 transition"
                        >
                          {t("dashboard.editMembers")}
                        </Link>
                      </div>
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
