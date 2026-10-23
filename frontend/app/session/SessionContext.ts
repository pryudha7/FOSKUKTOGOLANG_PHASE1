"use client";

import { createContext, useContext } from "react";
import type { Me } from "@/lib/api";

// The session operator of the page, provided by DesktopShell once known.
export const SessionContext = createContext<Me | null>(null);

export function useOperator(): Me {
  const me = useContext(SessionContext);
  if (!me) {
    throw new Error("useOperator outside DesktopShell");
  }
  return me;
}
