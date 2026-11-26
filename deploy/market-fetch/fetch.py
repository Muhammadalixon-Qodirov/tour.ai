"""market-fetch — O'zbekiston marketlaridan sahifa oladi: curl (brauzer TLS izi — OLX, Uzum kabi himoyalangan
saytlar Go mijozini bloklaydi, curl ni emas), O'zbekiston proksisi orqali (MARKET_PROXY_SOCKS).

POST /fetch  {"url","accept"?,"method"?(GET),"headers"?{},"body"?}  ->  {"status":200,"body":"..."}
GET  /health -> {"ok": true}

Faqat ruxsat etilgan market hostlari. Ichki compose tarmog'ida ishlaydi, tashqariga ochilmaydi.
"""
import json, os, subprocess, tempfile
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse

SOCKS = os.environ.get("MARKET_PROXY_SOCKS", "")   # "market-proxy:1080"; bo'sh — to'g'ridan-to'g'ri
UA = os.environ.get("MARKET_UA", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
                                 "(KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
ALLOW = {"www.olx.uz", "olx.uz", "asaxiy.uz", "www.asaxiy.uz", "api.olcha.uz", "olcha.uz",
         "gw.texnomart.uz", "texnomart.uz", "api.idea.uz", "idea.uz",
         "api.uzum.uz", "id.uzum.uz", "graphql.uzum.uz", "uzum.uz"}
TIMEOUT = 15
MAXBODY = 5 * 1024 * 1024


def fetch(spec):
    url = spec.get("url", "")
    host = (urlparse(url).hostname or "").lower()
    if urlparse(url).scheme != "https" or host not in ALLOW:
        return {"error": "host ruxsat etilmagan: " + host}
    method = (spec.get("method") or "GET").upper()
    if method not in ("GET", "POST"):
        return {"error": "method"}
    with tempfile.NamedTemporaryFile(delete=False) as tf:
        body_path = tf.name
    try:
        cmd = ["curl", "-s", "-m", str(TIMEOUT), "--compressed", "-A", UA,
               "-H", "Accept: " + (spec.get("accept") or "*/*"),
               "-H", "Accept-Language: uz,ru;q=0.9",
               "-o", body_path, "-w", "%{http_code}"]
        if SOCKS:
            cmd += ["--socks5-hostname", SOCKS]
        for k, v in (spec.get("headers") or {}).items():
            if isinstance(k, str) and isinstance(v, str) and "\n" not in k + v:
                cmd += ["-H", f"{k}: {v}"]
        if method == "POST":
            cmd += ["-X", "POST", "--data-raw", spec.get("body") or ""]
        cmd.append(url)
        p = subprocess.run(cmd, capture_output=True, timeout=TIMEOUT + 5)
        code = p.stdout.decode(errors="replace").strip() or "0"
        with open(body_path, "rb") as f:
            raw = f.read(MAXBODY + 1)
        if len(raw) > MAXBODY:
            return {"error": "javob juda katta"}
        try:
            status = int(code)
        except ValueError:
            status = 0
        if status == 0:
            return {"error": "curl: " + p.stderr.decode(errors="replace")[:160]}
        return {"status": status, "body": raw.decode("utf-8", "replace")}
    finally:
        try:
            os.unlink(body_path)
        except OSError:
            pass


class H(BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def reply(self, code, obj):
        b = json.dumps(obj, ensure_ascii=False).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(b)))
        self.end_headers()
        self.wfile.write(b)

    def do_GET(self):
        self.reply(200, {"ok": True}) if self.path == "/health" else self.reply(404, {"error": "not_found"})

    def do_POST(self):
        if self.path != "/fetch":
            return self.reply(404, {"error": "not_found"})
        n = int(self.headers.get("Content-Length") or 0)
        if n <= 0 or n > 64 * 1024:
            return self.reply(400, {"error": "so'rov"})
        try:
            spec = json.loads(self.rfile.read(n))
        except ValueError:
            return self.reply(400, {"error": "json"})
        try:
            self.reply(200, fetch(spec))
        except subprocess.TimeoutExpired:
            self.reply(200, {"error": "timeout"})


if __name__ == "__main__":
    print("market-fetch: :8890 socks=" + (SOCKS or "(yo'q)"), flush=True)
    ThreadingHTTPServer(("0.0.0.0", 8890), H).serve_forever()
