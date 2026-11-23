import { ExtractResponse } from "./types";

function joinUrl(base: string, segment: string): string {
  return `${base.replace(/\/+$/, "")}/${segment.replace(/^\/+/, "")}`;
}

async function parseError(res: Response): Promise<string> {
  try {
    const data = (await res.json()) as { error?: string };
    if (data?.error) return data.error;
  } catch {
    // fallback to status code
  }
  return `Request failed (${res.status})`;
}

export async function uploadInvoicePdfs(
  apiBaseUrl: string,
  files: File[]
): Promise<ExtractResponse> {
  const formData = new FormData();
  for (const file of files) {
    formData.append("files", file);
  }
  const res = await fetch(joinUrl(apiBaseUrl, "extract"), {
    method: "POST",
    body: formData,
  });
  if (!res.ok) {
    throw new Error(await parseError(res));
  }
  return (await res.json()) as ExtractResponse;
}
