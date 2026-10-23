"use client";

import { useEffect, useState } from "react";

// Empty Dashboard shell: shown only to an authenticated operator; anyone
// else is sent to the login page.
export default function Dashboard({ endpointPath, loginPath }: { endpointPath: string; loginPath: string }) {
  const [ready, setReady] = useState(false);

  useEffect(() => {
    fetch(`${endpointPath}/api/me`, { cache: "no-store" })
      .then((res) => {
        if (res.ok) {
          setReady(true);
        } else {
          window.location.assign(loginPath);
        }
      })
      .catch(() => window.location.assign(loginPath));
  }, [endpointPath, loginPath]);

  return <main aria-busy={!ready} />;
}
