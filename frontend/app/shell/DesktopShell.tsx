"use client";

import type { ReactNode } from "react";
import { useSession } from "@/app/session/useSession";
import IdleWarning from "@/app/session/IdleWarning";
import { SessionContext } from "@/app/session/SessionContext";
import Sidebar, { type MenuKey } from "./Sidebar";
import styles from "./shell.module.css";

// UI-008 main layout for the desktop endpoint: sidebar on the left, content
// on the right, and the idle warning (FR-ACCESS-005) for operators it applies
// to. Nothing renders until the session is known.
export default function DesktopShell({
  endpointPath,
  loginPath,
  current,
  layout = "page",
  children,
}: {
  endpointPath: string;
  loginPath: string;
  current: MenuKey;
  // "case": Case View fills the content area edge to edge with no "FOS"
  // title, as on Gambar 17.
  layout?: "page" | "case";
  children?: ReactNode;
}) {
  const me = useSession(endpointPath, loginPath);
  if (!me) {
    return <main aria-busy="true" />;
  }
  return (
    <SessionContext.Provider value={me}>
    <div className={styles.shell}>
      <div className={styles.sidebarSlot}>
        <Sidebar me={me} endpointPath={endpointPath} current={current} />
      </div>
      <main className={layout === "case" ? styles.contentCase : styles.content}>
        {/* UI-008: "FOS" title at the top left of the content area. */}
        {layout === "page" && <h1 className={styles.title}>FOS</h1>}
        {children}
      </main>
      {me.idle && <IdleWarning endpointPath={endpointPath} loginPath={me.loginPath} idle={me.idle} />}
    </div>
    </SessionContext.Provider>
  );
}
