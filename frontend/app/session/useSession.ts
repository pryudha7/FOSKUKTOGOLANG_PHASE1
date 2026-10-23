"use client";

import { useEffect, useState } from "react";
import { apiFetch, type Me } from "@/lib/api";

// Loads the session operator of an endpoint; without a session the operator
// is sent to that endpoint's login page (FR-ACCESS-004 rule 2).
export function useSession(endpointPath: string, loginPath: string): Me | null {
  const [me, setMe] = useState<Me | null>(null);

  useEffect(() => {
    apiFetch(`${endpointPath}/api/me`)
      .then(async (res) => {
        if (res.ok) {
          setMe((await res.json()) as Me);
        } else {
          window.location.assign(loginPath);
        }
      })
      .catch(() => window.location.assign(loginPath));
  }, [endpointPath, loginPath]);

  return me;
}
