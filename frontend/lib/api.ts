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

// An assignment of a case on an endpoint.
export type AssignmentRef = { endpointPath: string; caseId: string; assignmentId: number };

export type MoveResult<E> =
  | { kind: "moved" } // the browser is opening the next assignment
  | { kind: "invalid"; errors: E[] }
  | { kind: "failed" };

// moveAssignment posts a tabbed-flow move (submit or back) and, when it
// passes, opens the next assignment directly (FR-APP-003 rule 6).
export async function moveAssignment<E>(ref: AssignmentRef, action: "submit" | "back", body?: unknown): Promise<MoveResult<E>> {
  const url = `${ref.endpointPath}/api/cases/${encodeURIComponent(ref.caseId)}/assignments/${ref.assignmentId}/${action}`;
  const res = await apiFetch(url, {
    method: "POST",
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  }).catch(() => null);
  if (res?.ok) {
    const next = (await res.json()) as { assignmentId: number };
    window.location.assign(assignmentPath(ref.endpointPath, ref.caseId, next.assignmentId));
    return { kind: "moved" };
  }
  if (res?.status === 422) {
    return { kind: "invalid", errors: ((await res.json()) as { errors: E[] }).errors };
  }
  return { kind: "failed" };
}

// Ends the endpoint's session on the server, then opens its login page.
export async function logOff(endpointPath: string, loginPath: string): Promise<void> {
  await apiFetch(`${endpointPath}/api/logout`, { method: "POST" }).catch(() => undefined);
  window.location.assign(loginPath);
}
