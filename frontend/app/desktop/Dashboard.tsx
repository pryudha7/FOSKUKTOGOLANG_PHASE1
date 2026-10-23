"use client";

import DesktopShell from "@/app/shell/DesktopShell";

// UI-009 Dashboard inside the main layout; Home is the current menu. Its
// components arrive in later tickets (FR-DASH-002 to FR-DASH-004).
export default function Dashboard({ endpointPath, loginPath }: { endpointPath: string; loginPath: string }) {
  return <DesktopShell endpointPath={endpointPath} loginPath={loginPath} current="home" />;
}
