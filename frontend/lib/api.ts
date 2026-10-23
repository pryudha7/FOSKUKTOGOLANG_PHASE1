"use client";

// Every request to the server is operator activity (FR-ACCESS-005 rule 1).
// apiFetch announces each one so the idle timer can restart.
export const serverRequestEvent = "fos:server-request";

export function apiFetch(input: string, init?: RequestInit): Promise<Response> {
  window.dispatchEvent(new Event(serverRequestEvent));
  return fetch(input, { cache: "no-store", ...init });
}

// The session operator as returned by GET <endpoint>/api/me.
export type Me = {
  userId: string;
  fullName: string;
  // FR-ACCESS-005 timings, or null when the operator is exempt.
  idle: { warnAfterSeconds: number; logoffAfterSeconds: number } | null;
  loginPath: string;
  // Parent case types the operator may create (FR-NAV-003); empty hides Create.
  caseTypes: CaseType[];
};

// code is the stored case type (e.g. "Loan Financing"); label is shown ("Financing").
export type CaseType = { code: string; label: string };

// Case View routes on the desktop endpoint (FR-CASE-008).
export function assignmentPath(endpointPath: string, caseId: string, assignmentId: number): string {
  return `${endpointPath}/case/${encodeURIComponent(caseId)}/assignment/${assignmentId}`;
}

// Ends the endpoint's session on the server, then opens its login page.
export async function logOff(endpointPath: string, loginPath: string): Promise<void> {
  await apiFetch(`${endpointPath}/api/logout`, { method: "POST" }).catch(() => undefined);
  window.location.assign(loginPath);
}
