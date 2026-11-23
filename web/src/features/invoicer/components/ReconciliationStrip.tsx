"use client";

import { formatMoney } from "../transforms";
import { ReconciliationSummary } from "../types";

interface ReconciliationStripProps {
  summary: ReconciliationSummary;
  loading: boolean;
}

export function ReconciliationStrip({
  summary,
  loading,
}: ReconciliationStripProps) {
  if (loading) {
    return (
      <section
        aria-label="Reconciliation summary loading"
        className="grid grid-cols-2 lg:grid-cols-4 gap-px overflow-hidden rounded-lg border border-zinc-200 bg-zinc-200"
      >
        {[0, 1, 2, 3].map((idx) => (
          <div key={idx} className="bg-white p-4">
            <div className="h-3 w-24 animate-pulse rounded bg-zinc-200" />
            <div className="mt-2.5 h-7 w-32 animate-pulse rounded bg-zinc-200" />
            <div className="mt-2 h-3 w-36 animate-pulse rounded bg-zinc-100" />
          </div>
        ))}
      </section>
    );
  }

  return (
    <section
      aria-label="Reconciliation summary"
      className="grid grid-cols-2 lg:grid-cols-4 gap-px overflow-hidden rounded-lg border border-zinc-200 bg-zinc-200"
    >
      <div className="bg-white p-4">
        <p className="text-[11px] font-medium text-zinc-500">Extracted Records</p>
        <p className="mt-1 font-mono text-xl sm:text-2xl font-semibold tracking-tight text-zinc-950 tabular-nums whitespace-nowrap">
          {summary.recordCount}
        </p>
        <p className="mt-1 font-mono text-[11px] text-zinc-500 tabular-nums">
          {summary.pageCount} parsed invoice {summary.pageCount === 1 ? "page" : "pages"}
        </p>
      </div>

      <div className="bg-white p-4">
        <p className="text-[11px] font-medium text-zinc-500">Gross Trade Value</p>
        <p className="mt-1 font-mono text-xl sm:text-2xl font-semibold tracking-tight text-zinc-950 tabular-nums whitespace-nowrap">
          {formatMoney(summary.grossTradeValue)} EGP
        </p>
        <p className="mt-1 text-[11px] text-zinc-500">
          Executed value before charges
        </p>
      </div>

      <div className="bg-white p-4">
        <p className="text-[11px] font-medium text-zinc-500">
          Total Commission &amp; Fees
        </p>
        <p className="mt-1 font-mono text-xl sm:text-2xl font-semibold tracking-tight text-zinc-950 tabular-nums whitespace-nowrap">
          {formatMoney(summary.totalCommission)} EGP
        </p>
        <p className="mt-1 text-[11px] text-zinc-500">
          Exchange, regulatory, stamp &amp; brokerage
        </p>
      </div>

      <div className="bg-white p-4">
        <p className="text-[11px] font-medium text-zinc-500">Net Grand Total</p>
        <p className="mt-1 font-mono text-xl sm:text-2xl font-semibold tracking-tight text-zinc-950 tabular-nums whitespace-nowrap">
          {formatMoney(summary.netGrandTotal)} EGP
        </p>
        {summary.allReconciled ? (
          <p className="mt-1 font-mono text-[11px] font-medium text-emerald-700 tabular-nums">
            Reconciled · Variance {formatMoney(summary.maxVariance)} EGP
          </p>
        ) : (
          <p className="mt-1 font-mono text-[11px] font-medium text-red-700 tabular-nums">
            Check Required · Variance {formatMoney(summary.maxVariance)} EGP
          </p>
        )}
      </div>
    </section>
  );
}
