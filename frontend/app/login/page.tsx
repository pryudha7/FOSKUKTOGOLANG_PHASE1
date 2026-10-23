import { requireEnv } from "@/lib/env";
import LoginForm from "./LoginForm";

// UI-001 Login, served at LOGIN_PATH_DB and LOGIN_PATH_LDAP through
// middleware.ts; the form submits back to the path it was opened on.
export const dynamic = "force-dynamic";

export default async function LoginPage({ searchParams }: { searchParams: Promise<{ via?: string }> }) {
  const { via } = await searchParams;
  const loginPath = requireEnv(via === "ldap" ? "LOGIN_PATH_LDAP" : "LOGIN_PATH_DB");
  return <LoginForm loginPath={loginPath} endpointPath={requireEnv("ENDPOINT_PATH_DESKTOP")} />;
}
