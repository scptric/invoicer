import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { NextRequest, NextResponse } from "next/server";
import { getRepoRoot, runInvoicerJSON } from "@/lib/invoicer-server";

export const runtime = "nodejs";

const MAX_FILE_BYTES = 10 * 1024 * 1024; // 10 MB per PDF

interface GoInvoice {
  source_file: string;
  page_number: number;
  broker: string;
  [key: string]: unknown;
}

interface GoRecord {
  source_file?: string;
  [key: string]: unknown;
}

interface GoAuditReport {
  total_files: number;
  total_pages: number;
  brokers: Record<string, number>;
  reconciled_passed: number;
  reconciled_warned: number;
  total_rows_extracted: number;
  skipped_duplicates: number;
  total_rows_output: number;
  warnings?: string[];
  records: GoRecord[];
  invoices: GoInvoice[];
}

function normalizeSourceNames(
  report: GoAuditReport,
  pathToDisplay: Map<string, string>
): GoAuditReport {
  const normalizedInvoices = (report.invoices ?? []).map((inv) => {
    const resolved = path.resolve(/*turbopackIgnore: true*/ inv.source_file);
    const displayName =
      pathToDisplay.get(resolved) ??
      pathToDisplay.get(inv.source_file) ??
      path.basename(inv.source_file);
    return {
      ...inv,
      source_file: displayName,
    };
  });

  const normalizedRecords = (report.records ?? []).map((rec) => {
    const rawSource = rec.source_file ?? "";
    const resolved = rawSource ? path.resolve(/*turbopackIgnore: true*/ rawSource) : "";
    const displayName = rawSource
      ? (pathToDisplay.get(resolved) ??
        pathToDisplay.get(rawSource) ??
        path.basename(rawSource))
      : undefined;
    return {
      ...rec,
      source_file: displayName,
    };
  });

  return {
    ...report,
    records: normalizedRecords,
    invoices: normalizedInvoices,
  };
}

export async function POST(req: NextRequest) {
  let tempDir = "";

  try {
    const contentType = req.headers.get("content-type") ?? "";
    if (!contentType.includes("multipart/form-data")) {
      return NextResponse.json(
        { error: "Expected multipart/form-data containing PDF invoice files." },
        { status: 400 }
      );
    }

    const formData = await req.formData();
    const rawFiles = [
      ...formData.getAll("files"),
      ...formData.getAll("file"),
    ].filter((item): item is File => item instanceof File);

    if (rawFiles.length === 0) {
      return NextResponse.json(
        { error: "Please attach at least one Telda or Thndr E-Invoice PDF." },
        { status: 400 }
      );
    }

    const repoRoot = await getRepoRoot();
    tempDir = await fs.mkdtemp(path.join(os.tmpdir(), "invoicer-ui-"));
    const pathToDisplay = new Map<string, string>();
    const pdfPaths: string[] = [];

    for (let i = 0; i < rawFiles.length; i++) {
      const file = rawFiles[i];
      const fileName = file.name || `invoice-${i + 1}.pdf`;
      if (!fileName.toLowerCase().endsWith(".pdf")) {
        return NextResponse.json(
          { error: `Unsupported file "${fileName}". Only .pdf E-Invoices are accepted.` },
          { status: 400 }
        );
      }
      if (file.size === 0 || file.size > MAX_FILE_BYTES) {
        return NextResponse.json(
          { error: `File "${fileName}" must be between 1 byte and 10 MB.` },
          { status: 400 }
        );
      }

      const bytes = Buffer.from(await file.arrayBuffer());
      if (!bytes.subarray(0, 5).equals(Buffer.from("%PDF-"))) {
        return NextResponse.json(
          { error: `File "${fileName}" does not contain a valid %PDF- header.` },
          { status: 400 }
        );
      }

      const safePath = path.resolve(/*turbopackIgnore: true*/ tempDir, `invoice-${i}.pdf`);
      await fs.writeFile(safePath, bytes);
      pdfPaths.push(safePath);
      pathToDisplay.set(safePath, fileName);
    }

    const rawReport = await runInvoicerJSON<GoAuditReport>(pdfPaths, repoRoot);
    const report = normalizeSourceNames(rawReport, pathToDisplay);
    return NextResponse.json({ report });
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : "Failed to extract PDF invoice.";
    return NextResponse.json({ error: message }, { status: 422 });
  } finally {
    if (tempDir) {
      await fs.rm(tempDir, { recursive: true, force: true }).catch(() => {});
    }
  }
}
