// The login paths (API-001 LDAP, API-002 Database): each serves UI-001 and
// takes that path's credential submit.
export function loginPaths(): string[] {
  return [requireEnv("LOGIN_PATH_DB"), requireEnv("LOGIN_PATH_LDAP")];
}

// Runtime configuration (SoT §4.2). Every key is required; there are no defaults.
export function requireEnv(key: "LOGIN_PATH_DB" | "LOGIN_PATH_LDAP" | "ENDPOINT_PATH_DESKTOP" | "BACKEND_URL"): string {
  const value = process.env[key]?.trim();
  if (!value) {
    throw new Error(`missing configuration: ${key}`);
  }
  return value;
}
