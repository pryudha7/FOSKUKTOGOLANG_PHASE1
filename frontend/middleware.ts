import { NextResponse, type NextRequest } from "next/server";
import { defaultEndpoint, endpointForLoginPath, endpoints, requireEnv } from "@/lib/env";

// Maps the configured public paths onto the internal routes, and proxies the
// backend's routes to the Go backend: credential submits to the login paths
// (API-001, API-002) and each endpoint's API.
export function middleware(req: NextRequest) {
  const all = Object.values(endpoints());
  const { pathname, search } = req.nextUrl;
  const toBackend = () => NextResponse.rewrite(new URL(pathname + search, requireEnv("BACKEND_URL")));

  // FR-ACCESS-003 rule 3: the bare host goes to ENDPOINT_DEFAULT.
  if (pathname === "/") {
    return NextResponse.redirect(new URL(defaultEndpoint().path, req.url));
  }
  if (endpointForLoginPath(pathname)) {
    if (req.method === "POST") {
      return toBackend();
    }
    const page = new URL("/login", req.url);
    page.searchParams.set("path", pathname);
    return NextResponse.rewrite(page);
  }
  for (const ep of all) {
    if (pathname.startsWith(`${ep.path}/api/`)) {
      return toBackend();
    }
    // The endpoint home and its pages, e.g. <desktop>/case/LF-1/assignment/7.
    if (pathname === ep.path || pathname.startsWith(`${ep.path}/`)) {
      return NextResponse.rewrite(new URL(ep.route + pathname.slice(ep.path.length) + search, req.url));
    }
  }
  // The internal routes are reachable only through their configured paths.
  if (pathname === "/login" || all.some((ep) => pathname === ep.route || pathname.startsWith(`${ep.route}/`))) {
    return new NextResponse(null, { status: 404 });
  }
  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/|favicon.ico).*)"],
};
