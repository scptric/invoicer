"use client";

import { AlertCircle, Trash2, Upload } from "lucide-react";
import { useRef, useState } from "react";

interface InvoiceDropzoneProps {
  hasData: boolean;
  loading: boolean;
  error: string | null;
  statusNote: string | null;
  onFilesSelected: (files: File[]) => void;
  onClearAll: () => void;
}

export function InvoiceDropzone({
  hasData,
  loading,
  error,
  statusNote,
  onFilesSelected,
  onClearAll,
}: InvoiceDropzoneProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);

  function handleInputChange(e: React.ChangeEvent<HTMLInputElement>) {
    const selected = e.target.files ? Array.from(e.target.files) : [];
    if (selected.length > 0) {
      onFilesSelected(selected);
    }
    e.target.value = "";
  }

  function handleDrop(e: React.DragEvent<HTMLDivElement>) {
    e.preventDefault();
    setDragging(false);
    const dropped = Array.from(e.dataTransfer.files);
    if (dropped.length > 0) {
      onFilesSelected(dropped);
    }
  }

  const fileInput = (
    <input
      ref={inputRef}
      type="file"
      accept="application/pdf,.pdf"
      multiple
      onChange={handleInputChange}
      className="sr-only"
      aria-label="Upload Telda or Thndr E-Invoice PDFs"
    />
  );

  if (!hasData) {
    return (
      <section className="rounded-lg border border-zinc-200 bg-white p-6 sm:p-8">
        {fileInput}
        <div
          onDragOver={(e) => {
            e.preventDefault();
            setDragging(true);
          }}
          onDragLeave={() => setDragging(false)}
          onDrop={handleDrop}
          className={`rounded-md border border-dashed px-6 py-10 text-center transition-colors ${
            dragging
              ? "border-zinc-900 bg-zinc-100"
              : "border-zinc-300 bg-zinc-50"
          }`}
        >
          <Upload
            className="mx-auto h-6 w-6 text-zinc-500"
            aria-hidden="true"
          />
          <h2 className="mt-3 font-display text-sm font-semibold text-zinc-950">
            Upload E-Invoice PDFs
          </h2>
          <p className="mt-1 text-xs text-zinc-600">
            Drag and drop one or more PDF files here, or browse from your computer (max 10 MB per file).
          </p>
          <p className="mt-1.5 text-xs font-medium text-zinc-800">
            Currently supports E-Invoices from Telda and Thndr only.
          </p>

          <div className="mt-5 flex justify-center">
            <button
              type="button"
              onClick={() => inputRef.current?.click()}
              disabled={loading}
              className="inline-flex items-center gap-2 rounded-md bg-zinc-900 px-4 py-2 text-xs font-medium text-white transition-colors hover:bg-zinc-800 disabled:cursor-not-allowed disabled:bg-zinc-400 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900"
            >
              <Upload className="h-3.5 w-3.5" aria-hidden="true" />
              <span>{loading ? "Extracting Transactions..." : "Select PDF Files"}</span>
            </button>
          </div>
        </div>

        {error && (
          <div
            role="alert"
            className="mt-4 flex items-start gap-2 rounded-md border border-red-200 bg-red-50 p-3 text-xs text-red-800"
          >
            <AlertCircle
              className="mt-0.5 h-3.5 w-3.5 shrink-0 text-red-700"
              aria-hidden="true"
            />
            <span className="leading-relaxed break-words">{error}</span>
          </div>
        )}
      </section>
    );
  }

  return (
    <section className="rounded-lg border border-zinc-200 bg-white p-4">
      {fileInput}
      <div
        onDragOver={(e) => {
          e.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={handleDrop}
        className={`flex flex-col gap-3 rounded-md border border-dashed px-4 py-3 transition-colors sm:flex-row sm:items-center sm:justify-between ${
          dragging
            ? "border-zinc-900 bg-zinc-100"
            : "border-zinc-300 bg-zinc-50"
        }`}
      >
        <div className="flex items-start gap-3 sm:items-center">
          <Upload
            className="mt-0.5 h-4 w-4 shrink-0 text-zinc-500 sm:mt-0"
            aria-hidden="true"
          />
          <div>
            <p className="text-xs font-medium text-zinc-900">
              Drop more PDFs to append · Currently supports E-Invoices from Telda and Thndr only.
            </p>
            {statusNote && (
              <p className="mt-0.5 font-mono text-[11px] text-zinc-500">
                {statusNote}
              </p>
            )}
          </div>
        </div>

        <div className="flex shrink-0 items-center gap-2">
          <button
            type="button"
            onClick={() => inputRef.current?.click()}
            disabled={loading}
            className="inline-flex items-center gap-1.5 rounded-md border border-zinc-300 bg-white px-3 py-1.5 text-xs font-medium text-zinc-900 transition-colors hover:bg-zinc-100 disabled:opacity-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900"
          >
            <Upload className="h-3.5 w-3.5" aria-hidden="true" />
            <span>{loading ? "Extracting..." : "Add More PDFs"}</span>
          </button>
          <button
            type="button"
            onClick={onClearAll}
            disabled={loading}
            className="inline-flex items-center gap-1.5 rounded-md border border-zinc-200 bg-white px-3 py-1.5 text-xs font-medium text-zinc-600 transition-colors hover:bg-zinc-100 hover:text-zinc-900 disabled:opacity-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900"
          >
            <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
            <span>Clear All</span>
          </button>
        </div>
      </div>

      {error && (
        <div
          role="alert"
          className="mt-3 flex items-start gap-2 rounded-md border border-red-200 bg-red-50 p-3 text-xs text-red-800"
        >
          <AlertCircle
            className="mt-0.5 h-3.5 w-3.5 shrink-0 text-red-700"
            aria-hidden="true"
          />
          <span className="leading-relaxed break-words">{error}</span>
        </div>
      )}
    </section>
  );
}
