export type BrokerName = "telda" | "thndr" | "generic";
export type TradeSide = "Buy" | "Sell";

export interface Fill {
  transaction_no: string;
  quantity: number;
  price: number;
  value: number;
}

export interface FeeItem {
  name: string;
  amount: number;
}

export interface ReconciliationCheck {
  name: string;
  expected: number;
  actual: number;
  diff: number;
  passed: boolean;
  message?: string;
}

export interface ReconciliationResult {
  passed: boolean;
  checks: ReconciliationCheck[];
}

export interface InvoicePage {
  source_file: string;
  page_number: number;
  broker: BrokerName;
  customer_name?: string;
  custodian?: string;
  trade_date: string;
  currency?: string;
  security_name: string;
  raw_symbol_code: string;
  resolved_symbol: string;
  side: TradeSide;
  average_cost?: number;
  fills: Fill[];
  total_quantity: number;
  average_price: number;
  total_value: number;
  fees: FeeItem[];
  total_fees: number;
  grand_total: number;
  reconciliation: ReconciliationResult;
  warnings?: string[];
}

export interface GoCsvRecord {
  symbol: string;
  side: string;
  qty: number;
  fill_price?: number;
  commission?: number;
  closing_time: string;
  source_file?: string;
  page_number?: number;
  broker?: BrokerName;
  transaction_no?: string;
}

export interface AuditReport {
  total_files: number;
  total_pages: number;
  brokers: Record<string, number>;
  reconciled_passed: number;
  reconciled_warned: number;
  total_rows_extracted: number;
  skipped_duplicates: number;
  total_rows_output: number;
  warnings?: string[];
  records: GoCsvRecord[];
  invoices: InvoicePage[];
}

export interface ExtractResponse {
  report: AuditReport;
}

export type WorkspaceTab = "extractor" | "csv";
export type RowGranularity = "execution" | "invoice";

export interface ExtractedRow {
  id: string;
  symbol: string;
  side: TradeSide;
  qty: number;
  fillPrice: number;
  commission: number;
  closingTime: string;
  grossValue: number;
  totalAmount: number;
  vendorLabel: string;
  securityName: string;
  rawSymbolCode: string;
  broker: BrokerName;
  pageNumber: number;
  sourceFile: string;
  transactionNo: string;
  variance: number;
  reconciled: boolean;
  invoice: InvoicePage;
}

export interface ReconciliationSummary {
  recordCount: number;
  pageCount: number;
  grossTradeValue: number;
  totalCommission: number;
  netGrandTotal: number;
  maxVariance: number;
  allReconciled: boolean;
}

export interface SchemaMappingRow {
  column: string;
  format: string;
  extractionRule: string;
}
