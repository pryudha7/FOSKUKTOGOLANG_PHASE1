import { NextResponse, type NextRequest } from "next/server";
import { requireEnv } from "@/lib/env";

// Maps the configured public paths onto the internal routes, and proxies the
// endpoint API to the Go backend.
export function middleware(req: NextRequest) {
  const loginPath = requireEnv("LOGIN_PATH_DB");
  const desktopPath = requireEnv("ENDPOINT_PATH_DESKTOP");
  const { pathname, search } = req.nextUrl;

  if (pathname.startsWith(`${desktopPath}/api/`)) {
    return NextResponse.rewrite(new URL(pathname + search, requireEnv("BACKEND_URL")));
  }
  if (pathname === loginPath) {
    return NextResponse.rewrite(new URL("/login", req.url));
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
