"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { api, GroupMembersResponse, Member } from "@/lib/bridge";
import { t, sevaTypeLabel } from "@/lib/strings";
import { BackIcon, PlusIcon, SaveIcon, TrashIcon } from "@/components/devotional-icons";

export function GroupMembersClient({ sevaType, groupNo }: { sevaType: string; groupNo: number }) {
  const [data, setData] = useState<GroupMembersResponse | null>(null);
  const [draft, setDraft] = useState<Member[]>([]);
  const [loading, setLoading] = useState(true);
  const [toast, setToast] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    void load();
  }, [sevaType, groupNo]);

  async function load() {
    setToast(null);
    setLoading(true);
    try {
      const d = await api<GroupMembersResponse>(
        `/api/admin/v1/groups/${encodeURIComponent(sevaType)}/${groupNo}/members`
      );
      setData(d);
      setDraft(d.members);
    } catch (e) {
      setToast(e instanceof Error ? e.message : t("groupMembers.failedToLoad"));
    } finally {
      setLoading(false);
    }
  }

  const stats = useMemo(() => {
    const total = draft.length;
    const withPhone = draft.filter((m) => (m.phone_number ?? "").trim() !== "").length;
    return { total, withPhone };
  }, [draft]);

  function setRow(i: number, patch: Partial<Member>) {
    setDraft((prev) => prev.map((m, idx) => (idx === i ? { ...m, ...patch } : m)));
  }

  function addRow() {
    setDraft((prev) => [...prev, { name: "", adhyay_no: 1, phone_number: "" }]);
  }

  function removeRow(i: number) {
    setDraft((prev) => prev.filter((_, idx) => idx !== i));
  }

  async function save() {
    if (!data) return;
    setBusy(true);
    setToast(null);

    try {
      const clean = draft
        .map((m) => ({
          name: m.name.trim(),
          adhyay_no: Number(m.adhyay_no),
          phone_number: (m.phone_number ?? "").trim() || undefined
        }))
        .filter((m) => m.name !== "" && Number.isFinite(m.adhyay_no) && m.adhyay_no > 0);

      const resp = await api<any>(
        `/api/admin/v1/groups/${encodeURIComponent(sevaType)}/${groupNo}/members`,
        {
          method: "PUT",
          body: JSON.stringify({ expected_hash: data.hash, members: clean })
        }
      );

      setToast(t("groupMembers.saved"));
      if (resp?.hash) {
        const next = await api<GroupMembersResponse>(
          `/api/admin/v1/groups/${encodeURIComponent(sevaType)}/${groupNo}/members`
        );
        setData(next);
        setDraft(next.members);
      }
    } catch (e) {
      setToast(e instanceof Error ? e.message : t("groupMembers.saveFailed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-4">
      <div className="rounded-2xl bg-panel/60 border border-black/10 shadow-soft p-5">
        <div className="flex items-start justify-between gap-4">
          <div>
            <Link href="/groups" className="inline-flex items-center gap-2 text-sm text-muted hover:text-text">
              <BackIcon className="h-4 w-4" /> {t("groupMembers.back")}
            </Link>
            <h1 className="mt-2 text-xl font-semibold tracking-tight">
              {t("groupMembers.titlePrefix")} · {sevaTypeLabel(sevaType)} / {t("dashboard.group")} {groupNo}
            </h1>
            <div className="mt-1 text-sm text-muted">
              {t("groupMembers.total")}: {stats.total} · {t("groupMembers.withPhone")}: {stats.withPhone}
            </div>
            {data ? (
              <div className="mt-1 text-xs text-muted truncate">
                {t("groupMembers.csv")}: {data.csv_path}
              </div>
            ) : null}
          </div>
          <div className="flex gap-2">
            <button
              onClick={addRow}
              disabled={busy}
              className="inline-flex items-center gap-2 rounded-lg bg-black/5 hover:bg-black/10 border border-black/10 px-3 py-2 text-sm transition disabled:opacity-60"
            >
              <PlusIcon className="h-4 w-4" /> {t("groupMembers.add")}
            </button>
            <button
              onClick={save}
              disabled={busy || !data}
              className="inline-flex items-center gap-2 rounded-lg bg-brand px-3 py-2 text-sm font-medium hover:bg-brand/90 transition disabled:opacity-60"
            >
              <SaveIcon className="h-4 w-4" /> {t("groupMembers.save")}
            </button>
          </div>
        </div>
      </div>

      {toast ? (
        <div className="rounded-xl bg-black/5 border border-black/10 px-4 py-3 text-sm">{toast}</div>
      ) : null}

      {loading ? (
        <div className="text-sm text-muted">{t("groupMembers.loading")}</div>
      ) : (
        <div className="rounded-2xl bg-panel/40 border border-black/10 shadow-soft overflow-hidden">
          <div className="grid grid-cols-12 gap-2 px-4 py-3 text-xs text-muted border-b border-black/10">
            <div className="col-span-4">{t("groupMembers.headers.name")}</div>
            <div className="col-span-2">{t("groupMembers.headers.adhyay")}</div>
            <div className="col-span-5">{t("groupMembers.headers.phone")}</div>
            <div className="col-span-1 text-right"> </div>
          </div>
          <div className="divide-y divide-black/10">
            {draft.map((m, i) => (
              <div key={i} className="grid grid-cols-12 gap-2 px-4 py-3 items-center">
                <div className="col-span-12 md:col-span-4">
                  <input
                    value={m.name}
                    onChange={(e) => setRow(i, { name: e.target.value })}
                    className="w-full rounded-lg bg-panel2/80 border border-black/10 px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-brand/60"
                    placeholder={t("groupMembers.placeholders.memberName")}
                  />
                </div>
                <div className="col-span-6 md:col-span-2">
                  <input
                    type="number"
                    value={m.adhyay_no}
                    onChange={(e) => setRow(i, { adhyay_no: Number(e.target.value) })}
                    className="w-full rounded-lg bg-panel2/80 border border-black/10 px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-brand/60"
                    min={1}
                  />
                </div>
                <div className="col-span-6 md:col-span-5">
                  <input
                    value={m.phone_number ?? ""}
                    onChange={(e) => setRow(i, { phone_number: e.target.value })}
                    className="w-full rounded-lg bg-panel2/80 border border-black/10 px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-brand/60"
                    placeholder={t("groupMembers.placeholders.phoneOptional")}
                  />
                </div>
                <div className="col-span-12 md:col-span-1 flex justify-end">
                  <button
                    onClick={() => removeRow(i)}
                    className="inline-flex items-center justify-center h-9 w-9 rounded-lg bg-black/5 hover:bg-black/10 border border-black/10 transition"
                    title={t("groupMembers.remove")}
                  >
                    <TrashIcon className="h-4 w-4 text-danger" />
                  </button>
                </div>
              </div>
            ))}
            {draft.length === 0 ? (
              <div className="px-4 py-8 text-sm text-muted">{t("groupMembers.empty")}</div>
            ) : null}
          </div>
        </div>
      )}
    </div>
  );
}
