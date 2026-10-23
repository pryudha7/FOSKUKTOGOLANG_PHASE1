import { requireEnv } from "@/lib/env";
import LoginForm from "./LoginForm";

// UI-001 Login. Served at LOGIN_PATH_DB through middleware.ts.
export const dynamic = "force-dynamic";

export default function LoginPage() {
  return <LoginForm endpointPath={requireEnv("ENDPOINT_PATH_DESKTOP")} />;
}
