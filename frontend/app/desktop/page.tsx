import { endpoints } from "@/lib/env";
import Dashboard from "./Dashboard";

// UI-009 Dashboard, served at ENDPOINT_PATH_DESKTOP. Its components arrive in
// later tickets (FR-DASH-002 to FR-DASH-004).
export const dynamic = "force-dynamic";

export default function DesktopPage() {
  const { desktop } = endpoints();
  return <Dashboard endpointPath={desktop.path} loginPath={desktop.defaultLoginPath} />;
}
