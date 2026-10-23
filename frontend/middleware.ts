import { NextResponse, type NextRequest } from "next/server";
import { endpointForLoginPath, endpoints, requireEnv } from "@/lib/env";

// Maps the configured public paths onto the internal routes, and proxies the
// backend's routes to the Go backend: credential submits to the login paths
// (API-001, API-002) and each endpoint's API.
export function middleware(req: NextRequest) {
  const { desktop, mobile } = endpoints();
  const { pathname, search } = req.nextUrl;
  const toBackend = () => NextResponse.rewrite(new URL(pathname + search, requireEnv("BACKEND_URL")));

  // FR-ACCESS-003 rule 3: the bare host goes to ENDPOINT_DEFAULT.
  if (pathname === "/") {
    const target = requireEnv("ENDPOINT_DEFAULT") === "mobile" ? mobile : desktop;
    return NextResponse.redirect(new URL(target.path, req.url));
  }
  if (endpointForLoginPath(pathname)) {
    if (req.method === "POST") {
      return toBackend();
    }
    const page = new URL("/login", req.url);
    page.searchParams.set("path", pathname);
    return NextResponse.rewrite(page);
  }
  for (const [route, ep] of [["/desktop", desktop], ["/mobile", mobile]] as const) {
    if (pathname.startsWith(`${ep.path}/api/`)) {
      return toBackend();
    }
    if (pathname === ep.path) {
      return NextResponse.rewrite(new URL(route, req.url));
    }
  }
  // The internal routes are reachable only through their configured paths.
  if (["/login", "/desktop", "/mobile"].includes(pathname)) {
    return new NextResponse(null, { status: 404 });
  }
  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/|favicon.ico).*)"],
};
