import { notFound } from "next/navigation";
import { loginPaths, requireEnv } from "@/lib/env";
import LoginForm from "./LoginForm";

// UI-001 Login, served at each login path through middleware.ts; the form
// submits back to the path it was opened on.
export const dynamic = "force-dynamic";

export default async function LoginPage({ searchParams }: { searchParams: Promise<{ path?: string }> }) {
  const { path } = await searchParams;
  if (!path || !loginPaths().includes(path)) {
    notFound();
  }
  return <LoginForm loginPath={path} endpointPath={requireEnv("ENDPOINT_PATH_DESKTOP")} />;
}
