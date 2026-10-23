// Runtime configuration (SoT §4.2). Every key is required; there are no defaults.
export function requireEnv(key: "LOGIN_PATH_DB" | "ENDPOINT_PATH_DESKTOP" | "BACKEND_URL"): string {
  const value = process.env[key]?.trim();
  if (!value) {
    throw new Error(`missing configuration: ${key}`);
  }
  return value;
}
