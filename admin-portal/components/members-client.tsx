"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { api, GlobalMember } from "@/lib/bridge";
import { t, sevaTypeLabel } from "@/lib/strings";
import { SearchIcon } from "@/components/devotional-icons";

export function MembersClient() {
  const [members, setMembers] = useState<GlobalMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [q, setQ] = useState("");

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const data = await api<{ members: GlobalMember[] }>("/api/admin/v1/members");
        if (!cancelled) setMembers(data.members);
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : t("members.failedToLoad"));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const filtered = useMemo(() => {
    const s = q.trim().toLowerCase();
    if (!s) return members;
    return members.filter((m) => {
      const phone = (m.phone_number ?? "").toLowerCase();
      const name = (m.name ?? "").toLowerCase();
      return name.includes(s) || phone.includes(s);
    });
  }, [members, q]);

  return (
    <div className="space-y-4">
      <div className="rounded-2xl bg-panel/60 border border-black/10 shadow-soft p-5">
        <div className="flex items-start justify-between gap-4">
          <div>
            <div className="text-sm text-muted">{t("members.kicker")}</div>
            <h1 className="text-xl font-semibold tracking-tight">{t("members.title")}</h1>
            <p className="mt-1 text-sm text-muted">{t("members.subtitle")}</p>
          </div>
          <div className="w-full max-w-md">
            <div className="relative">
              <SearchIcon className="h-4 w-4 text-muted absolute left-3 top-1/2 -translate-y-1/2" />
              <input
                value={q}
                onChange={(e) => setQ(e.target.value)}
                placeholder={t("members.searchPlaceholder")}
                className="w-full rounded-lg bg-panel2/80 border border-black/10 pl-9 pr-3 py-2 text-sm outline-none focus:ring-2 focus:ring-brand/60"
              />
            </div>
          </div>
        </div>
      </div>

      {loading ? (
        <div className="text-sm text-muted">{t("members.loading")}</div>
      ) : error ? (
        <div className="text-sm text-danger">{error}</div>
      ) : (
        <div className="rounded-2xl bg-panel/40 border border-black/10 shadow-soft overflow-hidden">
          <div className="px-4 py-3 border-b border-black/10 text-xs text-muted">
            {t("members.showingPrefix")} {filtered.length} {t("members.of")} {members.length}
          </div>
          <div className="divide-y divide-black/10">
            {filtered.map((m) => (
              <div key={m.key} className="px-4 py-4">
                <div className="flex flex-col md:flex-row md:items-center justify-between gap-3">
                  <div>
                    <div className="font-semibold">{m.name || t("members.noName")}</div>
                    <div className="text-xs text-muted">{m.phone_number || t("members.noPhone")}</div>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {m.groups.map((g, idx) => (
                      <Link
                        key={`${m.key}:${idx}`}
                        href={`/groups/${encodeURIComponent(g.seva_type)}/${g.group_no}/members`}
                        className="inline-flex items-center gap-2 rounded-full bg-black/5 hover:bg-black/10 border border-black/10 px-3 py-1.5 text-xs transition"
                        title={g.group_name}
                      >
                        {sevaTypeLabel(g.seva_type)} · {g.group_no}
                      </Link>
                    ))}
                  </div>
                </div>
              </div>
            ))}
            {filtered.length === 0 ? (
              <div className="px-4 py-10 text-sm text-muted">{t("members.noMatches")}</div>
            ) : null}
          </div>
        </div>
      )}
    </div>
  );
}
