"use client";

import { useSession } from "@/app/session/useSession";

// No idle timeout on mobile: FR-ACCESS-005 applies to desktop (mobile is TBD-017).
export default function MobileHome({ endpointPath, loginPath }: { endpointPath: string; loginPath: string }) {
  const me = useSession(endpointPath, loginPath);
  return <main aria-busy={!me} />;
}
