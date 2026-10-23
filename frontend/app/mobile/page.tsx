import { endpoints } from "@/lib/env";
import MobileHome from "./MobileHome";

// Mobile endpoint home, served at ENDPOINT_PATH_MOBILE. Its functions are
// TBD-017; until then it is an empty page for an authenticated operator.
export const dynamic = "force-dynamic";

export default function MobilePage() {
  const { mobile } = endpoints();
  return <MobileHome endpointPath={mobile.path} loginPath={mobile.defaultLoginPath} />;
}
