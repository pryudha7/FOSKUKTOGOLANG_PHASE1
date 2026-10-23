// A local stand-in for Google Cloud Storage's JSON API and token endpoint,
// for manual UI checks only. It accepts any token and keeps objects in memory.
//
//   node fakegcs.js <port> <credential-file-to-write>
//
// Start the backend with STORAGE_EMULATOR_HOST=127.0.0.1:<port>,
// GCS_CREDENTIAL=<credential-file>, and any GCS_PROJECT_ID / GCS_BUCKET_NAME.
const http = require("http");
const fs = require("fs");
const crypto = require("crypto");

const port = Number(process.argv[2] || 14443);
const credFile = process.argv[3] || "shots/gcs.json";
const { privateKey } = crypto.generateKeyPairSync("rsa", { modulusLength: 2048 });
fs.writeFileSync(
  credFile,
  JSON.stringify({
    type: "service_account",
    client_email: "uicheck@local",
    private_key: privateKey.export({ type: "pkcs8", format: "pem" }),
    private_key_id: "uicheck",
    token_uri: `http://127.0.0.1:${port}/token`,
  }),
);

const objects = new Map();
http
  .createServer((req, res) => {
    const url = new URL(req.url, "http://x");
    const chunks = [];
    req.on("data", (c) => chunks.push(c));
    req.on("end", () => {
      if (url.pathname === "/token") {
        res.setHeader("Content-Type", "application/json");
        return res.end(JSON.stringify({ access_token: "uicheck", expires_in: 3600, token_type: "Bearer" }));
      }
      const raw = req.url.split("?")[0];
      if (req.method === "POST" && raw.startsWith("/upload/storage/v1/b/")) {
        objects.set(url.searchParams.get("name"), { type: req.headers["content-type"], data: Buffer.concat(chunks) });
        console.log("PUT", url.searchParams.get("name"));
        return res.end("{}");
      }
      const m = raw.match(/^\/storage\/v1\/b\/[^/]+\/o\/(.+)$/);
      const name = m && decodeURIComponent(m[1]);
      const obj = name && objects.get(name);
      if (!obj) {
        res.statusCode = 404;
        return res.end();
      }
      if (req.method === "DELETE") {
        objects.delete(name);
        console.log("DELETE", name);
        res.statusCode = 204;
        return res.end();
      }
      res.setHeader("Content-Type", obj.type || "application/octet-stream");
      res.end(obj.data);
    });
  })
  .listen(port, "127.0.0.1", () => console.log(`fake GCS on ${port}, credential ${credFile}`));
