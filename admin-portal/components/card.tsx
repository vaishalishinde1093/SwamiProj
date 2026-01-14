import Link from "next/link";
import { ChevronRightIcon } from "@/components/devotional-icons";

export function Card({
  title,
  description,
  href,
  icon
}: {
  title: string;
  description: string;
  href: string;
  icon: React.ReactNode;
}) {
  return (
    <Link
      href={href}
      className="group rounded-2xl bg-panel/70 border border-black/10 shadow-soft p-5 hover:bg-panel/90 transition"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-3">
          <div className="h-10 w-10 rounded-xl bg-black/5 border border-black/10 grid place-items-center">
            {icon}
          </div>
          <div>
            <div className="font-semibold tracking-tight">{title}</div>
            <div className="mt-1 text-sm text-muted">{description}</div>
          </div>
        </div>
        <ChevronRightIcon className="h-5 w-5 text-muted group-hover:text-text transition" />
      </div>
    </Link>
  );
}
