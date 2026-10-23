import { NextResponse, type NextRequest } from "next/server";
import { loginPaths, requireEnv } from "@/lib/env";

// Maps the configured public paths onto the internal routes, and proxies the
// backend's routes to the Go backend: credential submits to the login paths
// (API-001, API-002) and the endpoint API.
export function middleware(req: NextRequest) {
  const desktopPath = requireEnv("ENDPOINT_PATH_DESKTOP");
  const { pathname, search } = req.nextUrl;
  const toBackend = () => NextResponse.rewrite(new URL(pathname + search, requireEnv("BACKEND_URL")));

  if (loginPaths().includes(pathname)) {
    if (req.method === "POST") {
      return toBackend();
    }
    const page = new URL("/login", req.url);
    page.searchParams.set("path", pathname);
    return NextResponse.rewrite(page);
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
