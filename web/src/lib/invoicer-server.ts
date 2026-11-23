import { execFile } from "node:child_process";
import fs from "node:fs/promises";
import path from "node:path";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);

async function exists(target: string): Promise<boolean> {
  try {
    await fs.access(target);
    return true;
  } catch {
    return false;
  }
}

export async function getRepoRoot(): Promise<string> {
  const configured = process.env.INVOICER_ROOT_DIR?.trim();
  if (configured) {
    return path.resolve(configured);
  }
  const parent = path.resolve(process.cwd(), "..");
  if (
    (await exists(path.join(parent, "go.mod"))) ||
    (await exists(path.join(parent, "samples")))
  ) {
    return parent;
  }
  return process.cwd();
}

export async function getBinaryPath(repoRoot: string): Promise<string> {
  const configured = process.env.INVOICER_BIN_PATH?.trim();
  if (configured && (await exists(configured))) {
    return path.resolve(configured);
  }

  const candidates =
    process.platform === "win32"
      ? [
          path.join(repoRoot, "bin", "invoicer.exe"),
          path.join(process.cwd(), "bin", "invoicer.exe"),
          path.join(repoRoot, "bin", "invoicer"),
        ]
      : [
          path.join(repoRoot, "bin", "invoicer-linux"),
          path.join(repoRoot, "bin", "invoicer"),
          path.join(process.cwd(), "bin", "invoicer-linux"),
          path.join(process.cwd(), "bin", "invoicer"),
        ];

  for (const candidate of candidates) {
    if (await exists(candidate)) {
      return candidate;
    }
  }

  throw new Error(
    `Invoicer binary not found. Checked: ${candidates.join(", ")}. Run "go build -o bin/invoicer.exe ./cmd/invoicer" in the repository root.`
  );
}

export async function runInvoicerJSON<T = unknown>(
  pdfPaths: string[],
  repoRoot?: string
): Promise<T> {
  const root = repoRoot ?? (await getRepoRoot());
  const binPath = await getBinaryPath(root);
  const args = ["--json", "-", "-o", "none", "--sort", "desc", ...pdfPaths];

  try {
    const { stdout } = await execFileAsync(binPath, args, {
      cwd: root,
      timeout: 30_000,
      maxBuffer: 10 * 1024 * 1024,
    });
    return JSON.parse(stdout) as T;
  } catch (err: unknown) {
    if (err && typeof err === "object" && "stderr" in err) {
      const stderr = String((err as { stderr?: unknown }).stderr ?? "").trim();
      if (stderr) {
        throw new Error(stderr);
      }
    }
    throw err instanceof Error ? err : new Error("Failed running invoicer binary");
  }
}
