import type { Metadata } from "next";
import { JetBrains_Mono, Plus_Jakarta_Sans, Public_Sans } from "next/font/google";
import "./globals.css";

const plusJakarta = Plus_Jakarta_Sans({
  subsets: ["latin"],
  weight: ["500", "600", "700"],
  variable: "--font-jakarta",
  display: "swap",
});

const publicSans = Public_Sans({
  subsets: ["latin"],
  weight: ["400", "500", "600"],
  variable: "--font-body",
  display: "swap",
});

const jetbrainsMono = JetBrains_Mono({
  subsets: ["latin"],
  weight: ["400", "500", "600"],
  variable: "--font-jetbrains",
  display: "swap",
});

export const metadata: Metadata = {
  title: "Invoicer — PDF Invoice Extraction & Accounting Workbench",
  description:
    "Go-architected brokerage PDF invoice extraction engine and accounting CSV reconciliation workbench.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="en"
      className={`${plusJakarta.variable} ${publicSans.variable} ${jetbrainsMono.variable}`}
    >
      <body className="min-h-screen bg-zinc-50 font-sans text-zinc-900 antialiased selection:bg-zinc-900 selection:text-white">
        {children}
      </body>
    </html>
  );
}
