import type { NextConfig } from "next";

// Routing and backend addresses are read at runtime by middleware.ts, so a
// configuration change never needs a rebuild (SoT ENV-006).
const nextConfig: NextConfig = {
  poweredByHeader: false,
};

export default nextConfig;
