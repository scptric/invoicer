"use client";

import { Download } from "lucide-react";
import { WorkspaceTab } from "../types";

interface TopNavBarProps {
  activeTab: WorkspaceTab;
  onTabChange: (tab: WorkspaceTab) => void;
  onExportCsv: () => void;
  canExport: boolean;
}

const TABS: { id: WorkspaceTab; label: string }[] = [
  { id: "extractor", label: "Extractor" },
  { id: "csv", label: "CSV Output" },
];

export function TopNavBar({
  activeTab,
  onTabChange,
  onExportCsv,
  canExport,
}: TopNavBarProps) {
  return (
    <header className="sticky top-0 z-30 border-b border-zinc-200 bg-white">
      <div className="mx-auto max-w-[1440px] px-4 sm:px-6">
        <div className="flex h-14 items-center justify-between gap-4 md:gap-8">
          <div className="flex items-center gap-8">
            <span className="font-display text-base font-bold tracking-tight text-zinc-950 whitespace-nowrap">
              Invoicer
            </span>

            <nav aria-label="Workspace views" className="hidden md:flex items-center gap-6">
              {TABS.map((tab) => {
                const isActive = activeTab === tab.id;
                return (
                  <button
                    key={tab.id}
                    type="button"
                    onClick={() => onTabChange(tab.id)}
                    aria-current={isActive ? "page" : undefined}
                    className={`relative flex h-14 items-center px-0.5 text-xs font-medium whitespace-nowrap transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 ${
                      isActive
                        ? "text-zinc-950 font-semibold"
                        : "text-zinc-500 hover:text-zinc-900"
                    }`}
                  >
                    {tab.label}
                    {isActive && (
                      <span
                        aria-hidden="true"
                        className="absolute inset-x-0 bottom-0 h-0.5 bg-zinc-950"
                      />
                    )}
                  </button>
                );
              })}
            </nav>
          </div>

          <button
            type="button"
            onClick={onExportCsv}
            disabled={!canExport}
            className="inline-flex items-center gap-1.5 rounded-md bg-zinc-900 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-zinc-800 disabled:cursor-not-allowed disabled:bg-zinc-300 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900"
          >
            <Download className="h-3.5 w-3.5" aria-hidden="true" />
            <span>Export CSV</span>
          </button>
        </div>

        <nav
          aria-label="Mobile workspace views"
          className="grid grid-cols-2 border-t border-zinc-200 md:hidden"
        >
          {TABS.map((tab) => {
            const isActive = activeTab === tab.id;
            return (
              <button
                key={tab.id}
                type="button"
                onClick={() => onTabChange(tab.id)}
                aria-current={isActive ? "page" : undefined}
                className={`relative py-2.5 text-center text-xs font-medium whitespace-nowrap transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 ${
                  isActive
                    ? "text-zinc-950 font-semibold"
                    : "text-zinc-500 hover:text-zinc-900"
                }`}
              >
                {tab.label}
                {isActive && (
                  <span
                    aria-hidden="true"
                    className="absolute inset-x-3 bottom-0 h-0.5 bg-zinc-950"
                  />
                )}
              </button>
            );
          })}
        </nav>
      </div>
    </header>
  );
}
