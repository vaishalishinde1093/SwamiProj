import Link from "next/link";
import Image from "next/image";
import { t } from "@/lib/strings";
import { DeepakIcon, MandirIcon, OmIcon } from "@/components/devotional-icons";
import swamiBg from "../public/swami-bg.jpg";

const nav = [
  { href: "/dashboard", label: t("nav.dashboard"), icon: DeepakIcon },
  { href: "/groups", label: t("nav.groups"), icon: MandirIcon },
  { href: "/members", label: t("nav.members"), icon: OmIcon }
];

export function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen relative">
      <div className="fixed inset-0 -z-10">
        <Image
          src={swamiBg}
          alt=""
          fill
          priority
          className="object-cover object-center opacity-30 blur-xl scale-110"
        />
        <Image
          src={swamiBg}
          alt=""
          fill
          priority
          className="object-contain object-center opacity-95"
        />
        <div className="absolute inset-0 bg-bg/55" />
        <div className="absolute inset-0 bg-gradient-to-b from-bg/10 via-bg/25 to-bg/40" />
      </div>
      <div className="max-w-7xl mx-auto px-4 py-6">
        <div className="flex items-center justify-between gap-4">
          <Link href="/" className="flex items-center gap-2">
            <div className="h-9 w-9 rounded-xl bg-brand/15 border border-black/10 grid place-items-center shadow-soft">
              <OmIcon className="h-5 w-5 text-brand" />
            </div>
            <div>
              <div className="text-sm text-muted leading-none">{t("app.nameTop")}</div>
              <div className="text-base font-semibold tracking-tight">{t("app.name")}</div>
            </div>
          </Link>
          <div className="hidden md:flex items-center gap-2">
            {nav.map((n) => (
              <Link
                key={n.href}
                href={n.href}
                className="inline-flex items-center gap-2 rounded-lg px-3 py-2 text-sm bg-black/5 hover:bg-black/10 border border-black/10 transition"
              >
                <n.icon className="h-4 w-4 text-muted" />
                {n.label}
              </Link>
            ))}
          </div>
        </div>

        <div className="mt-6">{children}</div>

        <div className="mt-10 text-xs text-muted/80">
          {t("app.poweredBy")}
        </div>
      </div>
    </div>
  );
}
