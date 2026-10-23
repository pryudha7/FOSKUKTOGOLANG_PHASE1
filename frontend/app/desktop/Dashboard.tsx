"use client";

import { useSession } from "@/app/session/useSession";
import IdleWarning from "@/app/session/IdleWarning";

// Empty Dashboard shell for an authenticated desktop operator; its
// components arrive in later tickets.
export default function Dashboard({ endpointPath, loginPath }: { endpointPath: string; loginPath: string }) {
  const me = useSession(endpointPath, loginPath);
  return (
    <main aria-busy={!me}>
      {me?.idleTimeout && <IdleWarning endpointPath={endpointPath} loginPath={me.loginPath} />}
    </main>
  );
}
