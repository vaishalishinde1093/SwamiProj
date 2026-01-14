import Link from "next/link";
import { Card } from "@/components/card";
import { t } from "@/lib/strings";
import { ChevronRightIcon, DeepakIcon, MandirIcon, OmIcon } from "@/components/devotional-icons";

export default function HomePage() {
  return (
    <div className="space-y-6">
      <div className="rounded-2xl bg-panel/70 border border-black/10 shadow-soft p-6">
        <div className="flex items-start justify-between gap-4">
          <div>
            <div className="text-sm text-muted">{t("home.kicker")}</div>
            <h1 className="text-2xl font-semibold tracking-tight">{t("home.title")}</h1>
            <p className="mt-2 text-sm text-muted max-w-2xl">
              {t("home.subtitle")}
            </p>
          </div>
          <div className="flex gap-2">
            <Link
              href="/dashboard"
              className="inline-flex items-center gap-2 rounded-lg bg-brand px-4 py-2 text-sm font-medium text-white hover:bg-brand/90 transition"
            >
              {t("home.openDashboard")} <ChevronRightIcon className="h-4 w-4" />
            </Link>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card
          title={t("home.cards.operateSevas.title")}
          icon={<DeepakIcon className="h-5 w-5" />}
          description={t("home.cards.operateSevas.description")}
          href="/dashboard"
        />
        <Card
          title={t("home.cards.manageGroups.title")}
          icon={<MandirIcon className="h-5 w-5" />}
          description={t("home.cards.manageGroups.description")}
          href="/groups"
        />
        <Card
          title={t("home.cards.membersDirectory.title")}
          icon={<OmIcon className="h-5 w-5" />}
          description={t("home.cards.membersDirectory.description")}
          href="/members"
        />
      </div>
    </div>
  );
}
