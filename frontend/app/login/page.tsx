import { notFound } from "next/navigation";
import { endpointForLoginPath } from "@/lib/env";
import LoginForm from "./LoginForm";

// UI-001 Login, served at each login path through middleware.ts; the form
// submits back to the path it was opened on and then opens that endpoint.
export const dynamic = "force-dynamic";

export default async function LoginPage({ searchParams }: { searchParams: Promise<{ path?: string }> }) {
  const { path } = await searchParams;
  const endpoint = path ? endpointForLoginPath(path) : undefined;
  if (!path || !endpoint) {
    notFound();
  }
  return <LoginForm loginPath={path} endpointPath={endpoint.path} />;
}
