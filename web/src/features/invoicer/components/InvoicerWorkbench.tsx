"use client";

import { AnimatePresence, motion } from "motion/react";
import { useEffect, useMemo, useState } from "react";
import { uploadInvoicePdfs } from "../api";
import {
  appendUniqueInvoices,
  buildCsvOutput,
  computeReconciliationSummary,
  deriveExtractedRows,
  downloadCsv,
} from "../transforms";
import {
  AuditReport,
  ExtractedRow,
  InvoicePage,
  RowGranularity,
  WorkspaceTab,
} from "../types";
import { CsvPreviewPanel } from "./CsvPreviewPanel";
import { InvoiceDropzone } from "./InvoiceDropzone";
import { ReconciliationStrip } from "./ReconciliationStrip";
import { TopNavBar } from "./TopNavBar";
import { TransactionTable } from "./TransactionTable";

export interface InvoicerWorkbenchProps {
  /** Base URL for the Invoicer Next.js API routes. Defaults to "/api/invoicer". */
  apiBaseUrl?: string;
  /** Optional callback invoked whenever PDF invoices are extracted or cleared. */
  onTransactionsExtracted?: (rows: ExtractedRow[], report: AuditReport | null) => void;
}

export function InvoicerWorkbench({
  apiBaseUrl = "/api/invoicer",
  onTransactionsExtracted,
}: InvoicerWorkbenchProps) {
  const [activeTab, setActiveTab] = useState<WorkspaceTab>("extractor");
  const [granularity, setGranularity] = useState<RowGranularity>("execution");

  const [invoices, setInvoices] = useState<InvoicePage[]>([]);
  const [lastReport, setLastReport] = useState<AuditReport | null>(null);
  const [extractLoading, setExtractLoading] = useState(false);
  const [extractError, setExtractError] = useState<string | null>(null);
  const [statusNote, setStatusNote] = useState<string | null>(null);

  const extractedRows = useMemo(
    () => deriveExtractedRows(invoices, granularity),
    [invoices, granularity]
  );
  const summary = useMemo(
    () => computeReconciliationSummary(extractedRows, invoices),
    [extractedRows, invoices]
  );
  const csvOutput = useMemo(
    () => (extractedRows.length > 0 ? buildCsvOutput(extractedRows) : ""),
    [extractedRows]
  );

  useEffect(() => {
    if (onTransactionsExtracted) {
      onTransactionsExtracted(extractedRows, lastReport);
    }
  }, [extractedRows, lastReport, onTransactionsExtracted]);

  async function handleFilesSelected(selectedFiles: File[]) {
    const invalid = selectedFiles.find(
      (f) => !f.name.toLowerCase().endsWith(".pdf")
    );
    if (invalid) {
      setExtractError(
        `Invalid file "${invalid.name}". Please upload a valid .pdf brokerage E-Invoice.`
      );
      return;
    }

    setExtractLoading(true);
    setExtractError(null);
    try {
      const data = await uploadInvoicePdfs(apiBaseUrl, selectedFiles);
      const merged = appendUniqueInvoices(invoices, data.report.invoices ?? []);
      setInvoices(merged.invoices);
      setLastReport(data.report);

      const pageNote = `${merged.invoices.length} invoice ${
        merged.invoices.length === 1 ? "page" : "pages"
      } loaded`;
      if (merged.skippedPages > 0) {
        setStatusNote(
          `${pageNote} · ${merged.addedPages} added, ${merged.skippedPages} duplicate skipped`
        );
      } else {
        setStatusNote(pageNote);
      }
    } catch (err: unknown) {
      setExtractError(
        err instanceof Error ? err.message : "PDF extraction failed."
      );
    } finally {
      setExtractLoading(false);
    }
  }

  function handleClearAll() {
    setInvoices([]);
    setLastReport(null);
    setExtractError(null);
    setStatusNote(null);
  }

  function handleExportCsv() {
    if (!csvOutput) return;
    downloadCsv("transactions.csv", csvOutput);
  }

  const hasData = invoices.length > 0;

  return (
    <div className="flex min-h-screen flex-col bg-zinc-50">
      <TopNavBar
        activeTab={activeTab}
        onTabChange={setActiveTab}
        onExportCsv={handleExportCsv}
        canExport={extractedRows.length > 0}
      />

      <main className="mx-auto w-full max-w-[1440px] flex-1 space-y-6 px-4 py-6 sm:px-6 sm:py-8">
        <div className="flex flex-col gap-1">
          <h1 className="font-display text-2xl font-semibold tracking-tight text-zinc-950 sm:text-3xl [text-wrap:balance]">
            E-Invoice Extractor and Accounting Workbench
          </h1>
          <p className="text-sm leading-relaxed text-zinc-600">
            Upload brokerage PDF invoices to extract reconciled transactions in Standard Accounting Format. Currently supports E-Invoices from Telda and Thndr only.
          </p>
        </div>

        <AnimatePresence mode="wait">
          <motion.div
            key={activeTab}
            initial={{ opacity: 0, y: 4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -2 }}
            transition={{ duration: 0.16, ease: [0.16, 1, 0.3, 1] }}
            className="space-y-6"
          >
            {activeTab === "extractor" && (
              <>
                <InvoiceDropzone
                  hasData={hasData}
                  loading={extractLoading}
                  error={extractError}
                  statusNote={statusNote}
                  onFilesSelected={handleFilesSelected}
                  onClearAll={handleClearAll}
                />

                {(extractLoading || hasData) && (
                  <>
                    <ReconciliationStrip summary={summary} loading={extractLoading && !hasData} />
                    <TransactionTable
                      rows={extractedRows}
                      granularity={granularity}
                      onGranularityChange={setGranularity}
                      loading={extractLoading}
                    />
                  </>
                )}
              </>
            )}

            {activeTab === "csv" && (
              <CsvPreviewPanel
                rows={extractedRows}
                granularity={granularity}
                onGoToExtractor={() => setActiveTab("extractor")}
              />
            )}
          </motion.div>
        </AnimatePresence>
      </main>

      <footer className="border-t border-zinc-200 bg-white">
        <div className="mx-auto flex max-w-[1440px] flex-col gap-2 px-4 py-4 text-xs text-zinc-500 sm:flex-row sm:items-center sm:justify-between sm:px-6">
          <span>Invoicer · Standard Accounting Format · Supports Telda and Thndr E-Invoices</span>
          <span className="font-mono text-[11px] tabular-nums">
            {summary.pageCount} invoice page(s) · {extractedRows.length} row(s) ready
          </span>
        </div>
      </footer>
    </div>
  );
}
