import { NextResponse, type NextRequest } from "next/server";
import { requireEnv } from "@/lib/env";

// Maps the configured public paths onto the internal routes, and proxies the
// backend's routes to the Go backend: credential submits to the login paths
// (API-001, API-002) and the endpoint API.
export function middleware(req: NextRequest) {
  const loginPaths: Record<string, string> = {
    [requireEnv("LOGIN_PATH_DB")]: "database",
    [requireEnv("LOGIN_PATH_LDAP")]: "ldap",
  };
  const desktopPath = requireEnv("ENDPOINT_PATH_DESKTOP");
  const { pathname, search } = req.nextUrl;
  const toBackend = () => NextResponse.rewrite(new URL(pathname + search, requireEnv("BACKEND_URL")));

  const via = loginPaths[pathname];
  if (via) {
    return req.method === "POST" ? toBackend() : NextResponse.rewrite(new URL(`/login?via=${via}`, req.url));
  }
  if (pathname.startsWith(`${desktopPath}/api/`)) {
    return toBackend();
  }
  if (pathname === desktopPath) {
    return NextResponse.rewrite(new URL("/desktop", req.url));
  }
  // The internal routes are reachable only through their configured paths.
  if (pathname === "/login" || pathname === "/desktop") {
    return new NextResponse(null, { status: 404 });
  }
  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/|favicon.ico).*)"],
};
