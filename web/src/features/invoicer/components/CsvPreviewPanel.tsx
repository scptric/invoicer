"use client";

import {
  ArrowRight,
  Check,
  Copy,
  Download,
  FileSpreadsheet,
  Search,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import {
  STANDARD_CSV_HEADER,
  brokerDisplayName,
  buildCsvOutput,
  downloadCsv,
  formatTrimmedNumber,
  getStandardSchemaMapping,
} from "../transforms";
import { ExtractedRow, RowGranularity } from "../types";

type SideFilter = "All" | "Buy" | "Sell";
type BrokerFilter = "all" | "telda" | "thndr";

const SIDE_OPTIONS: SideFilter[] = ["All", "Buy", "Sell"];
const BROKER_OPTIONS: { id: BrokerFilter; label: string }[] = [
  { id: "all", label: "All Brokers" },
  { id: "telda", label: "Telda" },
  { id: "thndr", label: "Thndr" },
];

export interface CsvPreviewPanelProps {
  rows: ExtractedRow[];
  granularity: RowGranularity;
  onGoToExtractor: () => void;
}

export function CsvPreviewPanel({
  rows,
  granularity,
  onGoToExtractor,
}: CsvPreviewPanelProps) {
  const [sideFilter, setSideFilter] = useState<SideFilter>("All");
  const [brokerFilter, setBrokerFilter] = useState<BrokerFilter>("all");
  const [searchQuery, setSearchQuery] = useState("");
  const [copied, setCopied] = useState(false);
  const copyTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (copyTimerRef.current) {
        clearTimeout(copyTimerRef.current);
      }
    };
  }, []);

  const filteredRows = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    return rows.filter((row) => {
      if (sideFilter !== "All" && row.side !== sideFilter) {
        return false;
      }
      if (brokerFilter !== "all" && row.broker !== brokerFilter) {
        return false;
      }
      if (!query) {
        return true;
      }
      const fields = [
        row.symbol,
        row.side,
        row.vendorLabel,
        brokerDisplayName(row.broker),
        row.securityName,
        row.rawSymbolCode,
        row.transactionNo,
        row.closingTime,
        row.sourceFile,
      ];
      return fields.some((field) => field.toLowerCase().includes(query));
    });
  }, [rows, sideFilter, brokerFilter, searchQuery]);

  const hasActiveFilters =
    sideFilter !== "All" ||
    brokerFilter !== "all" ||
    searchQuery.trim().length > 0;

  const filteredCsv = useMemo(
    () => (filteredRows.length > 0 ? buildCsvOutput(filteredRows) : ""),
    [filteredRows]
  );

  const csvLines = useMemo(
    () => (filteredCsv ? filteredCsv.trimEnd().split("\n") : []),
    [filteredCsv]
  );

  const schemaRows = useMemo(
    () => getStandardSchemaMapping(granularity),
    [granularity]
  );

  function handleResetFilters() {
    setSideFilter("All");
    setBrokerFilter("all");
    setSearchQuery("");
  }

  async function handleCopyCsv() {
    if (!filteredCsv) return;
    try {
      await navigator.clipboard.writeText(filteredCsv);
      setCopied(true);
      if (copyTimerRef.current) {
        clearTimeout(copyTimerRef.current);
      }
      copyTimerRef.current = setTimeout(() => {
        setCopied(false);
      }, 1800);
    } catch {
      // Clipboard API may be unavailable in restricted browser contexts
    }
  }

  function handleDownload() {
    if (!filteredCsv) return;
    downloadCsv("transactions.csv", filteredCsv);
  }

  return (
    <div className="space-y-6">
      {/* Interactive CSV Viewer */}
      <section className="overflow-hidden rounded-lg border border-zinc-200 bg-white">
        <div className="flex flex-col gap-3 border-b border-zinc-200 px-4 py-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 className="font-display text-base font-semibold tracking-tight text-zinc-950">
              CSV Viewer
            </h2>
            <p className="mt-0.5 font-mono text-xs text-zinc-500 tabular-nums">
              {rows.length > 0
                ? `Showing ${filteredRows.length} of ${rows.length} row(s) · ${
                    granularity === "execution"
                      ? "Execution fills"
                      : "Invoice summaries"
                  }`
                : "Standard Accounting Format · 6 columns"}
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            {hasActiveFilters && (
              <button
                type="button"
                onClick={handleResetFilters}
                className="rounded-md px-2.5 py-1.5 text-xs font-medium text-zinc-600 transition-colors hover:bg-zinc-100 hover:text-zinc-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900"
              >
                Clear filters
              </button>
            )}
            <button
              type="button"
              onClick={handleCopyCsv}
              disabled={!filteredCsv}
              className="inline-flex items-center gap-1.5 rounded-md border border-zinc-200 bg-white px-3 py-1.5 text-xs font-medium text-zinc-800 transition-colors hover:bg-zinc-50 disabled:cursor-not-allowed disabled:opacity-40 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900"
            >
              {copied ? (
                <>
                  <Check className="h-3.5 w-3.5 text-emerald-700" />
                  <span className="text-emerald-700">Copied</span>
                </>
              ) : (
                <>
                  <Copy className="h-3.5 w-3.5" />
                  <span>Copy CSV</span>
                </>
              )}
            </button>
            <button
              type="button"
              onClick={handleDownload}
              disabled={!filteredCsv}
              className="inline-flex items-center gap-1.5 rounded-md bg-zinc-900 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-zinc-800 disabled:cursor-not-allowed disabled:opacity-40 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900"
            >
              <Download className="h-3.5 w-3.5" />
              <span>Download CSV</span>
            </button>
          </div>
        </div>

        {/* Filter & Search Toolbar */}
        <div className="flex flex-col gap-3 border-b border-zinc-200 bg-zinc-50/50 px-4 py-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex flex-wrap items-center gap-2.5">
            <div
              role="group"
              aria-label="Filter by trade side"
              className="inline-flex items-center rounded-md bg-zinc-100 p-1"
            >
              {SIDE_OPTIONS.map((side) => {
                const active = sideFilter === side;
                return (
                  <button
                    key={side}
                    type="button"
                    aria-pressed={active}
                    onClick={() => setSideFilter(side)}
                    className={`rounded px-2.5 py-1 text-xs transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 ${
                      active
                        ? "bg-white font-medium text-zinc-950 shadow-xs"
                        : "text-zinc-600 hover:text-zinc-900"
                    }`}
                  >
                    {side}
                  </button>
                );
              })}
            </div>

            <div
              role="group"
              aria-label="Filter by broker"
              className="inline-flex items-center rounded-md bg-zinc-100 p-1"
            >
              {BROKER_OPTIONS.map((broker) => {
                const active = brokerFilter === broker.id;
                return (
                  <button
                    key={broker.id}
                    type="button"
                    aria-pressed={active}
                    onClick={() => setBrokerFilter(broker.id)}
                    className={`rounded px-2.5 py-1 text-xs transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 ${
                      active
                        ? "bg-white font-medium text-zinc-950 shadow-xs"
                        : "text-zinc-600 hover:text-zinc-900"
                    }`}
                  >
                    {broker.label}
                  </button>
                );
              })}
            </div>
          </div>

          <div className="relative w-full sm:w-64">
            <label htmlFor="csv-viewer-search" className="sr-only">
              Search CSV rows
            </label>
            <Search
              aria-hidden="true"
              className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-zinc-400"
            />
            <input
              id="csv-viewer-search"
              type="search"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search symbol, invoice, date..."
              className="h-8 w-full rounded-md border border-zinc-200 bg-white pl-8 pr-3 text-xs text-zinc-900 placeholder:text-zinc-400 focus:border-zinc-400 focus:outline-none"
            />
          </div>
        </div>

        {/* Viewer Table / Empty States */}
        {rows.length === 0 ? (
          <div className="flex flex-col items-center justify-center px-4 py-14 text-center">
            <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-lg border border-zinc-200 bg-zinc-50 text-zinc-500">
              <FileSpreadsheet className="h-5 w-5" />
            </div>
            <p className="text-sm font-medium text-zinc-900">
              No extracted transactions yet
            </p>
            <p className="mt-1 max-w-md text-xs leading-relaxed text-zinc-500">
              Upload Telda or Thndr PDF E-Invoices in the Extractor tab to inspect, filter, copy, and download CSV transactions.
            </p>
            <button
              type="button"
              onClick={onGoToExtractor}
              className="mt-4 inline-flex items-center gap-1.5 rounded-md bg-zinc-900 px-3.5 py-2 text-xs font-medium text-white transition-colors hover:bg-zinc-800 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900"
            >
              <span>Go to Extractor</span>
              <ArrowRight className="h-3.5 w-3.5" />
            </button>
          </div>
        ) : filteredRows.length === 0 ? (
          <div className="flex flex-col items-center justify-center px-4 py-12 text-center">
            <p className="text-sm font-medium text-zinc-900">
              No CSV rows match the current filter.
            </p>
            <p className="mt-1 text-xs text-zinc-500">
              Try clearing your search query or switching Side and Broker filters.
            </p>
            <button
              type="button"
              onClick={handleResetFilters}
              className="mt-3 rounded-md border border-zinc-200 bg-white px-3 py-1.5 text-xs font-medium text-zinc-800 transition-colors hover:bg-zinc-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900"
            >
              Reset filters
            </button>
          </div>
        ) : (
          <div className="max-h-[520px] overflow-auto">
            <table className="w-full border-collapse text-left text-xs">
              <thead className="sticky top-0 z-10 border-b border-zinc-200 bg-zinc-50 text-[11px] font-medium uppercase tracking-wider text-zinc-500">
                <tr>
                  <th scope="col" className="px-4 py-2.5">
                    Symbol
                  </th>
                  <th scope="col" className="px-4 py-2.5">
                    Side
                  </th>
                  <th scope="col" className="px-4 py-2.5 text-right">
                    Qty
                  </th>
                  <th scope="col" className="px-4 py-2.5 text-right">
                    Fill Price
                  </th>
                  <th scope="col" className="px-4 py-2.5 text-right">
                    Commission
                  </th>
                  <th scope="col" className="px-4 py-2.5">
                    Closing Time
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-100">
                {filteredRows.map((row) => (
                  <tr
                    key={row.id}
                    className="transition-colors hover:bg-zinc-50/80"
                  >
                    <td className="px-4 py-2.5 whitespace-nowrap">
                      <div className="flex items-baseline gap-2">
                        <span className="font-mono font-medium text-zinc-950">
                          {row.symbol}
                        </span>
                        <span className="text-[11px] text-zinc-500">
                          {brokerDisplayName(row.broker)}
                        </span>
                      </div>
                    </td>
                    <td className="px-4 py-2.5 whitespace-nowrap">
                      <span
                        className={
                          row.side === "Buy"
                            ? "font-medium text-emerald-700"
                            : "font-medium text-red-700"
                        }
                      >
                        {row.side}
                      </span>
                    </td>
                    <td className="px-4 py-2.5 text-right font-mono tabular-nums text-zinc-900">
                      {formatTrimmedNumber(row.qty, 4)}
                    </td>
                    <td className="px-4 py-2.5 text-right font-mono tabular-nums text-zinc-900">
                      {formatTrimmedNumber(row.fillPrice, 4)}
                    </td>
                    <td className="px-4 py-2.5 text-right font-mono tabular-nums text-zinc-600">
                      {formatTrimmedNumber(row.commission, 2)}
                    </td>
                    <td className="px-4 py-2.5 whitespace-nowrap font-mono tabular-nums text-zinc-600">
                      {row.closingTime}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {/* RFC 4180 CSV Preview */}
      <section className="overflow-hidden rounded-lg border border-zinc-800 bg-zinc-950">
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-zinc-800 px-4 py-2.5">
          <div className="flex items-center gap-2">
            <h2 className="text-xs font-medium text-zinc-200">
              RFC 4180 CSV Preview
            </h2>
            {hasActiveFilters && rows.length > 0 && (
              <span className="text-[11px] text-zinc-400">
                (filtered)
              </span>
            )}
          </div>
          <span className="font-mono text-[11px] text-zinc-400 tabular-nums">
            transactions.csv · {csvLines.length} line(s)
          </span>
        </div>

        <div className="max-h-[320px] overflow-auto p-4 font-mono text-xs leading-relaxed">
          {csvLines.length === 0 ? (
            <div className="space-y-1 text-zinc-500">
              <div className="flex gap-3">
                <span className="w-6 shrink-0 select-none text-right tabular-nums text-zinc-600">
                  1
                </span>
                <span className="text-zinc-400">{STANDARD_CSV_HEADER}</span>
              </div>
              <div className="flex gap-3">
                <span className="w-6 shrink-0 select-none text-right tabular-nums text-zinc-600">
                  2
                </span>
                <span>
                  {rows.length === 0
                    ? "# Upload Telda or Thndr E-Invoices to preview CSV output"
                    : "# No rows match the current filter"}
                </span>
              </div>
            </div>
          ) : (
            <div className="space-y-0.5">
              {csvLines.map((line, idx) => (
                <div key={`${idx}-${line}`} className="flex gap-3">
                  <span className="w-6 shrink-0 select-none text-right tabular-nums text-zinc-500">
                    {idx + 1}
                  </span>
                  <span
                    className={
                      idx === 0
                        ? "whitespace-pre font-medium text-white"
                        : "whitespace-pre text-zinc-200"
                    }
                  >
                    {line}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      </section>

      {/* Standard Accounting Format Schema Reference */}
      <section className="overflow-hidden rounded-lg border border-zinc-200 bg-white">
        <div className="border-b border-zinc-200 px-4 py-3.5">
          <h2 className="font-display text-sm font-semibold tracking-tight text-zinc-950">
            Standard Accounting Format Schema
          </h2>
          <p className="mt-0.5 text-xs text-zinc-500">
            Fixed 6-column accounting layout generated from Telda and Thndr E-Invoices ({granularity === "execution" ? "per execution fill" : "per invoice page"}).
          </p>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-left text-xs">
            <thead className="border-b border-zinc-200 bg-zinc-50 text-[11px] font-medium uppercase tracking-wider text-zinc-500">
              <tr>
                <th scope="col" className="px-4 py-2.5">
                  Column
                </th>
                <th scope="col" className="px-4 py-2.5">
                  Format
                </th>
                <th scope="col" className="px-4 py-2.5">
                  Extraction Rule
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-100">
              {schemaRows.map((item) => (
                <tr key={item.column}>
                  <td className="px-4 py-2.5 font-mono font-medium text-zinc-950 whitespace-nowrap">
                    {item.column}
                  </td>
                  <td className="px-4 py-2.5 font-mono text-zinc-600 whitespace-nowrap">
                    {item.format}
                  </td>
                  <td className="px-4 py-2.5 text-zinc-600">
                    {item.extractionRule}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
