"use client";

import { ChevronDown, ChevronRight, Search } from "lucide-react";
import { Fragment, useMemo, useState } from "react";
import { formatMoney, formatTrimmedNumber } from "../transforms";
import { ExtractedRow, RowGranularity } from "../types";

interface TransactionTableProps {
  rows: ExtractedRow[];
  granularity: RowGranularity;
  onGranularityChange: (next: RowGranularity) => void;
  loading: boolean;
}

export function TransactionTable({
  rows,
  granularity,
  onGranularityChange,
  loading,
}: TransactionTableProps) {
  const [query, setQuery] = useState("");
  const [expandedIds, setExpandedIds] = useState<Set<string>>(() => new Set());

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return rows;
    return rows.filter((r) => {
      return (
        r.symbol.toLowerCase().includes(q) ||
        r.vendorLabel.toLowerCase().includes(q) ||
        r.securityName.toLowerCase().includes(q) ||
        r.side.toLowerCase().includes(q) ||
        r.rawSymbolCode.toLowerCase().includes(q) ||
        r.transactionNo.toLowerCase().includes(q) ||
        r.closingTime.toLowerCase().includes(q)
      );
    });
  }, [rows, query]);

  function toggleRow(id: string) {
    setExpandedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }

  return (
    <section className="overflow-hidden rounded-lg border border-zinc-200 bg-white">
      <div className="flex flex-col gap-3 border-b border-zinc-200 px-4 py-3.5 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h2 className="font-display text-sm font-semibold text-zinc-950">
            Extracted Transactions
          </h2>
          <p className="mt-0.5 text-xs text-zinc-600">
            Standard Accounting Format · Expand any row to inspect itemized fees and reconciliation details.
          </p>
        </div>

        <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
          <div
            role="group"
            aria-label="Row granularity"
            className="inline-flex rounded-md bg-zinc-100 p-1"
          >
            {(
              [
                { id: "execution", label: "By Execution" },
                { id: "invoice", label: "By Invoice" },
              ] as { id: RowGranularity; label: string }[]
            ).map((opt) => {
              const active = granularity === opt.id;
              return (
                <button
                  key={opt.id}
                  type="button"
                  aria-pressed={active}
                  onClick={() => onGranularityChange(opt.id)}
                  className={`rounded px-2.5 py-1 text-xs font-medium whitespace-nowrap transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 ${
                    active
                      ? "bg-white text-zinc-950 shadow-2xs"
                      : "text-zinc-600 hover:text-zinc-900"
                  }`}
                >
                  {opt.label}
                </button>
              );
            })}
          </div>

          <div className="relative w-full sm:w-64">
            <label htmlFor="tx-search" className="sr-only">
              Filter extracted transactions
            </label>
            <Search
              className="pointer-events-none absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2 text-zinc-400"
              aria-hidden="true"
            />
            <input
              id="tx-search"
              type="search"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search symbol, security, broker..."
              className="w-full rounded-md border border-zinc-200 bg-zinc-50 py-1.5 pr-3 pl-8 text-xs text-zinc-900 placeholder:text-zinc-400 focus:border-zinc-400 focus:bg-white focus:outline-none focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900"
            />
          </div>
        </div>
      </div>

      {loading && rows.length === 0 ? (
        <div className="space-y-3 p-4">
          {[0, 1, 2, 3, 4].map((i) => (
            <div
              key={i}
              className="h-9 w-full animate-pulse rounded bg-zinc-100"
            />
          ))}
        </div>
      ) : filtered.length === 0 ? (
        <div className="px-6 py-10 text-center">
          <p className="text-xs text-zinc-600">
            No extracted rows match &ldquo;{query}&rdquo;.
          </p>
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-left text-xs">
            <thead className="border-b border-zinc-200 bg-zinc-50 text-[11px] font-medium text-zinc-500">
              <tr>
                <th scope="col" className="w-9 px-3 py-2.5">
                  <span className="sr-only">Expand row</span>
                </th>
                <th scope="col" className="px-3 py-2.5 whitespace-nowrap">
                  Symbol
                </th>
                <th scope="col" className="px-3 py-2.5 whitespace-nowrap">
                  Side
                </th>
                <th scope="col" className="px-3 py-2.5">
                  Security
                </th>
                <th scope="col" className="px-3 py-2.5 text-right whitespace-nowrap">
                  Qty
                </th>
                <th scope="col" className="px-3 py-2.5 text-right whitespace-nowrap">
                  Fill Price
                </th>
                <th scope="col" className="px-3 py-2.5 text-right whitespace-nowrap">
                  Commission
                </th>
                <th scope="col" className="px-3 py-2.5 text-right whitespace-nowrap">
                  Net Total
                </th>
                <th scope="col" className="px-3 py-2.5 whitespace-nowrap">
                  Closing Time
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-100">
              {filtered.map((row) => {
                const isOpen = expandedIds.has(row.id);
                return (
                  <Fragment key={row.id}>
                    <tr className="bg-white transition-colors hover:bg-zinc-50">
                      <td className="px-3 py-2.5 align-top">
                        <button
                          type="button"
                          onClick={() => toggleRow(row.id)}
                          aria-expanded={isOpen}
                          aria-label={`Toggle audit details for ${row.symbol}`}
                          className="inline-flex h-5 w-5 items-center justify-center rounded text-zinc-500 transition-colors hover:bg-zinc-100 hover:text-zinc-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900"
                        >
                          {isOpen ? (
                            <ChevronDown className="h-3.5 w-3.5" aria-hidden="true" />
                          ) : (
                            <ChevronRight className="h-3.5 w-3.5" aria-hidden="true" />
                          )}
                        </button>
                      </td>
                      <td className="px-3 py-2.5 align-top">
                        <p className="font-mono font-medium text-zinc-950 whitespace-nowrap">
                          {row.symbol}
                        </p>
                        <p className="mt-0.5 font-mono text-[11px] text-zinc-500 whitespace-nowrap">
                          {row.vendorLabel} · Page {row.pageNumber}
                        </p>
                      </td>
                      <td className="px-3 py-2.5 align-top text-zinc-900 whitespace-nowrap">
                        {row.side}
                      </td>
                      <td className="px-3 py-2.5 align-top">
                        <p className="text-zinc-900">{row.securityName}</p>
                        <p className="mt-0.5 font-mono text-[11px] text-zinc-500">
                          {row.transactionNo}
                        </p>
                      </td>
                      <td className="px-3 py-2.5 align-top text-right font-mono text-zinc-900 tabular-nums whitespace-nowrap">
                        {formatTrimmedNumber(row.qty, 4)}
                      </td>
                      <td className="px-3 py-2.5 align-top text-right font-mono text-zinc-900 tabular-nums whitespace-nowrap">
                        {formatTrimmedNumber(row.fillPrice, 4)}
                      </td>
                      <td className="px-3 py-2.5 align-top text-right font-mono text-zinc-700 tabular-nums whitespace-nowrap">
                        {formatTrimmedNumber(row.commission, 2)}
                      </td>
                      <td className="px-3 py-2.5 align-top text-right font-mono font-semibold text-zinc-950 tabular-nums whitespace-nowrap">
                        {formatMoney(row.totalAmount)} EGP
                      </td>
                      <td className="px-3 py-2.5 align-top font-mono text-[11px] text-zinc-700 tabular-nums whitespace-nowrap">
                        {row.closingTime}
                      </td>
                    </tr>

                    {isOpen && (
                      <tr className="bg-zinc-50">
                        <td colSpan={9} className="border-t border-zinc-100 px-6 py-4">
                          <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
                            <div>
                              <h3 className="font-display text-xs font-semibold text-zinc-950">
                                Statutory and Brokerage Fee Breakdown
                              </h3>
                              <table className="mt-2 w-full text-xs">
                                <tbody className="divide-y divide-zinc-200/70">
                                  {(row.invoice.fees ?? []).map((fee, idx) => (
                                    <tr key={`${fee.name}-${idx}`}>
                                      <td className="py-1.5 text-zinc-600">
                                        {fee.name}
                                      </td>
                                      <td className="py-1.5 text-right font-mono text-zinc-900 tabular-nums">
                                        {formatMoney(fee.amount)} EGP
                                      </td>
                                    </tr>
                                  ))}
                                  <tr className="border-t border-zinc-300 font-semibold text-zinc-950">
                                    <td className="pt-2">Total Deducted Fees</td>
                                    <td className="pt-2 text-right font-mono tabular-nums">
                                      {formatMoney(row.invoice.total_fees)} EGP
                                    </td>
                                  </tr>
                                </tbody>
                              </table>
                            </div>

                            <div className="space-y-4">
                              <div>
                                <h3 className="font-display text-xs font-semibold text-zinc-950">
                                  Audit Metadata and Reconciliation
                                </h3>
                                <dl className="mt-2 grid grid-cols-2 gap-x-4 gap-y-1.5 text-xs">
                                  <dt className="text-zinc-500">Source File</dt>
                                  <dd className="truncate text-right font-mono text-zinc-900">
                                    {row.sourceFile}
                                  </dd>
                                  <dt className="text-zinc-500">Raw Code / ISIN</dt>
                                  <dd className="text-right font-mono text-zinc-900">
                                    {row.rawSymbolCode || "—"}
                                  </dd>
                                  <dt className="text-zinc-500">Custodian</dt>
                                  <dd className="text-right text-zinc-900">
                                    {row.invoice.custodian || row.vendorLabel}
                                  </dd>
                                  <dt className="text-zinc-500">Account Holder</dt>
                                  <dd className="text-right text-zinc-900">
                                    {row.invoice.customer_name || "—"}
                                  </dd>
                                  <dt className="text-zinc-500">Page Gross Value</dt>
                                  <dd className="text-right font-mono text-zinc-900 tabular-nums">
                                    {formatMoney(row.invoice.total_value)} EGP
                                  </dd>
                                  <dt className="text-zinc-500">Reconciliation</dt>
                                  <dd
                                    className={`text-right font-mono font-medium tabular-nums ${
                                      row.reconciled
                                        ? "text-emerald-700"
                                        : "text-red-700"
                                    }`}
                                  >
                                    {row.reconciled
                                      ? "Balanced (0.00 EGP)"
                                      : `Variance ${formatMoney(row.variance)} EGP`}
                                  </dd>
                                </dl>
                              </div>

                              <div>
                                <h4 className="font-display text-xs font-semibold text-zinc-950">
                                  Underlying Execution Fills ({row.invoice.fills.length})
                                </h4>
                                <ul className="mt-1.5 divide-y divide-zinc-200/70 border-t border-zinc-200">
                                  {row.invoice.fills.map((fill) => (
                                    <li
                                      key={fill.transaction_no}
                                      className="flex items-center justify-between gap-2 py-1.5 font-mono text-[11px] text-zinc-700 tabular-nums"
                                    >
                                      <span>{fill.transaction_no}</span>
                                      <span>
                                        {formatTrimmedNumber(fill.quantity, 4)} @{" "}
                                        {formatTrimmedNumber(fill.price, 4)} ={" "}
                                        {formatMoney(fill.value)} EGP
                                      </span>
                                    </li>
                                  ))}
                                </ul>
                              </div>
                            </div>
                          </div>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
