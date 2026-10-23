// Runtime configuration (SoT §4.2). Every key is required; there are no defaults.
type Key =
  | "LOGIN_PATH_DB"
  | "LOGIN_PATH_LDAP"
  | "ENDPOINT_PATH_DESKTOP"
  | "ENDPOINT_PATH_MOBILE"
  | "ENDPOINT_DEFAULT"
  | "BACKEND_URL";

export function requireEnv(key: Key): string {
  const value = process.env[key]?.trim();
  if (!value) {
    throw new Error(`missing configuration: ${key}`);
  }
  return value;
}

// An endpoint (FR-ACCESS-003) and its login paths (API-001 LDAP, API-002
// Database). Mobile login paths sit under the mobile endpoint path (agreed
// for TBD-017); desktop keeps the bare LOGIN_PATH_* values.
export type Endpoint = { path: string; loginPaths: string[]; defaultLoginPath: string };

export function endpoints(): { desktop: Endpoint; mobile: Endpoint } {
  const db = requireEnv("LOGIN_PATH_DB");
  const ldap = requireEnv("LOGIN_PATH_LDAP");
  const desktop = requireEnv("ENDPOINT_PATH_DESKTOP");
  const mobile = requireEnv("ENDPOINT_PATH_MOBILE");
  return {
    desktop: { path: desktop, loginPaths: [db, ldap], defaultLoginPath: db },
    mobile: { path: mobile, loginPaths: [mobile + db, mobile + ldap], defaultLoginPath: mobile + db },
  };
}

// The endpoint whose login page is served at path, if any.
export function endpointForLoginPath(path: string): Endpoint | undefined {
  return Object.values(endpoints()).find((e) => e.loginPaths.includes(path));
}
