import {
  BrokerName,
  ExtractedRow,
  InvoicePage,
  ReconciliationSummary,
  RowGranularity,
  SchemaMappingRow,
} from "./types";

export const STANDARD_CSV_HEADER =
  "Symbol,Side,Qty,Fill Price,Commission,Closing Time";

export function round2(v: number): number {
  const r = Math.round(v * 100) / 100;
  return Object.is(r, -0) ? 0 : r;
}

export function round4(v: number): number {
  const r = Math.round(v * 10000) / 10000;
  return Object.is(r, -0) ? 0 : r;
}

/**
 * Formats numbers cleanly without padded trailing zeros (Standard Accounting Format).
 */
export function formatTrimmedNumber(v: number, maxDecimals: number): string {
  const factor = Math.pow(10, maxDecimals);
  const rounded = Math.round(v * factor) / factor;
  if (rounded === 0) return "0";
  let str = rounded.toFixed(maxDecimals);
  if (str.includes(".")) {
    str = str.replace(/0+$/, "").replace(/\.$/, "");
  }
  return str === "-0" || str === "" ? "0" : str;
}

export function formatMoney(v: number): string {
  const safe = round2(v);
  return safe.toLocaleString("en-US", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
}

export function brokerDisplayName(broker: BrokerName): string {
  switch (broker) {
    case "telda":
      return "Telda Securities";
    case "thndr":
      return "Thndr Securities";
    default:
      return "Generic Broker";
  }
}

/**
 * Allocates invoice total fees across execution fills in exact integer cents
 * using the Largest Remainder Method (mirrors internal/reconcile/reconcile.go).
 */
function allocateInvoiceFeesPerFill(invoice: InvoicePage): number[] {
  const fills = invoice.fills ?? [];
  const n = fills.length;
  if (n === 0) return [];
  if (n === 1) return [round2(invoice.total_fees)];

  let fixedPerFillCents = 0;
  for (const fee of invoice.fees ?? []) {
    const low = fee.name.toLowerCase();
    if (low.includes("efsa") || low.includes("fra")) {
      if (Math.abs(fee.amount - n) <= 0.01) {
        fixedPerFillCents = 100;
        break;
      }
    }
  }

  const totalCents = Math.round(invoice.total_fees * 100);
  const remainingCents = totalCents - fixedPerFillCents * n;
  const sign = remainingCents < 0 ? -1 : 1;
  const absRem = Math.abs(remainingCents);

  const weights = fills.map((f) => {
    const w = Math.abs(f.value) || Math.abs(f.quantity * f.price);
    return w > 0 ? w : Math.max(1, Math.abs(f.quantity));
  });
  const sumWeights = weights.reduce((acc, w) => acc + w, 0) || n;

  const floors: number[] = [];
  const remainders: { idx: number; rem: number }[] = [];
  let floorSum = 0;

  for (let i = 0; i < n; i++) {
    const exact = (absRem * weights[i]) / sumWeights;
    const fl = Math.floor(exact);
    floors.push(fl);
    floorSum += fl;
    remainders.push({ idx: i, rem: exact - fl });
  }

  remainders.sort((a, b) => (b.rem !== a.rem ? b.rem - a.rem : a.idx - b.idx));
  const leftover = absRem - floorSum;
  for (let k = 0; k < leftover && k < n; k++) {
    floors[remainders[k].idx] += 1;
  }

  return floors.map((fl) => (fixedPerFillCents + sign * fl) / 100);
}

function extractYMD(isoDate: string): string {
  const m = isoDate.match(/^(\d{4}-\d{2}-\d{2})/);
  return m ? m[1] : isoDate.slice(0, 10);
}

export function invoiceDedupKey(inv: InvoicePage): string {
  const txKey = (inv.fills ?? [])
    .map((f) => f.transaction_no)
    .sort()
    .join(",");
  return [
    inv.broker,
    extractYMD(inv.trade_date),
    inv.raw_symbol_code,
    inv.side,
    txKey,
  ].join("|");
}

export function appendUniqueInvoices(
  existing: InvoicePage[],
  incoming: InvoicePage[]
): { invoices: InvoicePage[]; addedPages: number; skippedPages: number } {
  const seen = new Set(existing.map(invoiceDedupKey));
  const combined = [...existing];
  let addedPages = 0;
  let skippedPages = 0;

  for (const inv of incoming) {
    const key = invoiceDedupKey(inv);
    if (seen.has(key)) {
      skippedPages += 1;
      continue;
    }
    seen.add(key);
    combined.push(inv);
    addedPages += 1;
  }

  return { invoices: combined, addedPages, skippedPages };
}

export function deriveExtractedRows(
  invoices: InvoicePage[],
  granularity: RowGranularity
): ExtractedRow[] {
  const rows: ExtractedRow[] = [];

  for (const inv of invoices) {
    const ymd = extractYMD(inv.trade_date);
    const closingTime = `${ymd} 00:00:00`;
    const vendorLabel = brokerDisplayName(inv.broker);
    const symbol = inv.resolved_symbol || inv.raw_symbol_code;

    const invGross =
      inv.total_value > 0
        ? inv.total_value
        : round2(inv.fills.reduce((s, f) => s + f.value, 0));
    const expectedGrand =
      inv.side === "Sell"
        ? round2(invGross - inv.total_fees)
        : round2(invGross + inv.total_fees);
    const pageVariance = round2(Math.abs(expectedGrand - inv.grand_total));
    const pageReconciled = Boolean(inv.reconciliation?.passed) && pageVariance <= 0.02;

    if (granularity === "invoice" || inv.fills.length === 0) {
      const qty =
        inv.total_quantity > 0
          ? inv.total_quantity
          : round4(inv.fills.reduce((s, f) => s + f.quantity, 0));
      const fillPrice = qty > 0 ? round4(invGross / qty) : round4(inv.average_price);
      const commission = round2(inv.total_fees);
      const txSummary =
        inv.fills.map((f) => f.transaction_no).join("+") || `page-${inv.page_number}`;

      rows.push({
        id: `${invoiceDedupKey(inv)}-order`,
        symbol,
        side: inv.side,
        qty: round4(qty),
        fillPrice,
        commission,
        closingTime,
        grossValue: round2(invGross),
        totalAmount: round2(inv.grand_total),
        vendorLabel,
        securityName: inv.security_name,
        rawSymbolCode: inv.raw_symbol_code,
        broker: inv.broker,
        pageNumber: inv.page_number,
        sourceFile: inv.source_file,
        transactionNo: txSummary,
        variance: pageVariance,
        reconciled: pageReconciled,
        invoice: inv,
      });
      continue;
    }

    const perFillFees = allocateInvoiceFeesPerFill(inv);
    inv.fills.forEach((fill, idx) => {
      const comm = round2(perFillFees[idx] ?? 0);
      const net =
        inv.side === "Sell" ? round2(fill.value - comm) : round2(fill.value + comm);

      rows.push({
        id: `${invoiceDedupKey(inv)}-fill-${idx}-${fill.transaction_no}`,
        symbol,
        side: inv.side,
        qty: round4(fill.quantity),
        fillPrice: round4(fill.price),
        commission: comm,
        closingTime,
        grossValue: round2(fill.value),
        totalAmount: net,
        vendorLabel,
        securityName: inv.security_name,
        rawSymbolCode: inv.raw_symbol_code,
        broker: inv.broker,
        pageNumber: inv.page_number,
        sourceFile: inv.source_file,
        transactionNo: fill.transaction_no,
        variance: pageVariance,
        reconciled: pageReconciled,
        invoice: inv,
      });
    });
  }

  // Sort newest first while keeping deterministic extraction order within the same date
  rows.sort((a, b) => b.closingTime.localeCompare(a.closingTime));
  return rows;
}

export function computeReconciliationSummary(
  rows: ExtractedRow[],
  invoices: InvoicePage[]
): ReconciliationSummary {
  let grossTradeValue = 0;
  let totalCommission = 0;
  let netGrandTotal = 0;

  for (const r of rows) {
    grossTradeValue += r.grossValue;
    totalCommission += r.commission;
    netGrandTotal += r.totalAmount;
  }

  let maxVariance = 0;
  let allReconciled = invoices.length > 0;
  for (const inv of invoices) {
    const gross =
      inv.total_value > 0
        ? inv.total_value
        : inv.fills.reduce((s, f) => s + f.value, 0);
    const expected =
      inv.side === "Sell" ? gross - inv.total_fees : gross + inv.total_fees;
    const diff = round2(Math.abs(expected - inv.grand_total));
    if (diff > maxVariance) maxVariance = diff;
    if (!inv.reconciliation?.passed || diff > 0.02) {
      allReconciled = false;
    }
  }

  return {
    recordCount: rows.length,
    pageCount: invoices.length,
    grossTradeValue: round2(grossTradeValue),
    totalCommission: round2(totalCommission),
    netGrandTotal: round2(netGrandTotal),
    maxVariance: round2(maxVariance),
    allReconciled,
  };
}

function escapeCsvCell(val: string): string {
  if (
    val.includes(",") ||
    val.includes('"') ||
    val.includes("\n") ||
    val.includes("\r")
  ) {
    return `"${val.replace(/"/g, '""')}"`;
  }
  return val;
}

export function buildCsvOutput(rows: ExtractedRow[]): string {
  const lines: string[] = [STANDARD_CSV_HEADER];
  for (const r of rows) {
    const cells = [
      r.symbol,
      r.side,
      formatTrimmedNumber(r.qty, 4),
      formatTrimmedNumber(r.fillPrice, 4),
      formatTrimmedNumber(r.commission, 2),
      r.closingTime,
    ];
    lines.push(cells.map(escapeCsvCell).join(","));
  }
  return lines.join("\n") + "\n";
}

export function getStandardSchemaMapping(
  granularity: RowGranularity
): SchemaMappingRow[] {
  return [
    {
      column: "Symbol",
      format: "EGX:<TICKER>",
      extractionRule:
        "Direct ticker (Telda) or Egyptian ISIN automatically mapped to EGX ticker (Thndr)",
    },
    {
      column: "Side",
      format: "Buy | Sell",
      extractionRule: "Normalized transaction side from invoice header",
    },
    {
      column: "Qty",
      format: "Unpadded decimal",
      extractionRule:
        granularity === "invoice"
          ? "Total executed quantity per invoice page"
          : "Individual execution fill quantity",
    },
    {
      column: "Fill Price",
      format: "Unpadded decimal",
      extractionRule:
        granularity === "invoice"
          ? "Weighted average execution price across page fills"
          : "Exact execution fill price per trade line",
    },
    {
      column: "Commission",
      format: "Unpadded decimal (2dp)",
      extractionRule:
        granularity === "invoice"
          ? "Total deducted invoice fees (exchange, MCDR, regulatory, stamp, brokerage)"
          : "Total fees allocated per fill with exact cent conservation",
    },
    {
      column: "Closing Time",
      format: "YYYY-MM-DD HH:MM:SS",
      extractionRule: "Normalized invoice trade date paired with 00:00:00 timestamp",
    },
  ];
}

export function downloadCsv(filename: string, content: string): void {
  const blob = new Blob([content], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}
