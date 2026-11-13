// Minimal static server for web/dist that applies the headers from _headers,
// so local runs and browser tests use the same CSP as production.
// Usage: node tests/web/serve.mjs <dir> <port>
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { extname, join, normalize, resolve } from "node:path";
import { pathToFileURL } from "node:url";

const TYPES = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".wasm": "application/wasm",
  ".svg": "image/svg+xml",
  ".json": "application/json",
};

// headersFor parses the "/*" section of a Cloudflare _headers file.
export async function headersFor(dir) {
  const text = await readFile(join(dir, "_headers"), "utf8");
  const headers = {};
  let inAll = false;
  for (const line of text.split("\n")) {
    if (/^\S/.test(line)) inAll = line.trim() === "/*";
    else if (inAll && line.includes(":")) {
      const i = line.indexOf(":");
      headers[line.slice(0, i).trim()] = line.slice(i + 1).trim();
    }
  }
  return headers;
}

export async function serve(dir, port) {
  const root = resolve(dir);
  const headers = await headersFor(root);
  const server = createServer(async (req, res) => {
    let path = decodeURIComponent(new URL(req.url, "http://localhost").pathname);
    if (path.endsWith("/")) path += "index.html";
    const file = normalize(join(root, path));
    if (!file.startsWith(root) || path.includes("_headers")) {
      res.writeHead(404).end();
      return;
    }
    try {
      const body = await readFile(file);
      res.writeHead(200, { ...headers, "Content-Type": TYPES[extname(file)] || "application/octet-stream" });
      res.end(body);
    } catch {
      res.writeHead(404).end();
    }
  });
  await new Promise((ok) => server.listen(port, "127.0.0.1", ok));
  return server;
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  const [dir = "web/dist", port = "8080"] = process.argv.slice(2);
  await serve(dir, Number(port));
  console.log(`serving ${dir} on http://localhost:${port}`);
}
