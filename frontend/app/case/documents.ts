"use client";

import { apiFetch } from "@/lib/api";
import type { StoredFile } from "./types";

export type DownloadResult = "downloaded" | "forbidden" | "failed";

// downloadFile fetches a case file through the server and hands it to the
// browser as a download under its file_name, for every type (FR-FILE-003,
// DEC-114). A refusal is reported so the page can show ERR-035.
export async function downloadFile(endpointPath: string, caseId: string, file: StoredFile): Promise<DownloadResult> {
  const url = `${endpointPath}/api/cases/${encodeURIComponent(caseId)}/documents/${encodeURIComponent(file.fileId)}`;
  const res = await apiFetch(url).catch(() => null);
  if (res?.status === 403) {
    return "forbidden";
  }
  if (!res?.ok) {
    return "failed";
  }
  const blob = await res.blob();
  const href = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = href;
  a.download = file.fileName;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(href), 60_000);
  return "downloaded";
}

// uploadPath is the Upload Document modal's Submit target (FR-FILE-002).
export function uploadPath(endpointPath: string, caseId: string, assignmentId: number): string {
  return `${endpointPath}/api/cases/${encodeURIComponent(caseId)}/assignments/${assignmentId}/documents`;
}

// The file types UI-011 accepts (agreed for TBD-027), for the file dialog.
export const acceptedTypes = ".jpg,.jpeg,.png,.gif,.bmp,.pdf,.xls,.xlsx,.csv,.rar,.zip";
