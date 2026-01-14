import "./globals.css";
import type { Metadata } from "next";
import { AppShell } from "@/components/app-shell";

export const metadata: Metadata = {
  title: "सेवा व्यवस्थापन पोर्टल",
  description: "सेवा, गट आणि सदस्य व्यवस्थापन"
};

export default function RootLayout({
  children
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="mr">
      <body>
        <AppShell>{children}</AppShell>
      </body>
    </html>
  );
}
