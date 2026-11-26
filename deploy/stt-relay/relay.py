"""stt-relay — serverdagi o'zbekcha STT modellari (Kotib) uchun ko'prik.

API konteyneri xostdagi 127.0.0.1 portlariga yeta olmaydi (firewall), shuning uchun bu xizmat xost tarmog'ida
ishlaydi va API bilan unix-socket orqali gaplashadi (tarmoq/firewall ishtirok etmaydi).

POST /stt   body: istalgan audio (webm/ogg/m4a/mp3/wav), sarlavha X-Max-Seconds (ixtiyoriy)
            -> {"text": "...", "engine": "kotib-gpu" | "kotib-cpu", "audioSeconds": 4.2}
GET  /health -> {"gpuDown": false, ...}

Dvigatellar (teskari SSH tunnellar orqali, LexSim bilan bir xil):
  GPU Kotib  127.0.0.1:8093  model "Kotib (tinish belgilari bilan)"  (5 s audio ~1 s)
  CPU Kotib  127.0.0.1:7009  (sekinroq, zaxira)
GPU ishlamasa 30 s davomida unga urinilmaydi ("o'chirgich"). Faqat standart kutubxona + ffmpeg.
"""
import json, os, socket, socketserver, subprocess, threading, time, urllib.request, uuid
from http.server import BaseHTTPRequestHandler

SOCK = os.environ.get("STT_SOCKET", "/run/stt/stt.sock")
HOST = os.environ.get("STT_HOST", "127.0.0.1")
ENGINES = [
    {"name": "kotib-gpu", "port": int(os.environ.get("STT_GPU_PORT", "8093")), "model": "Kotib (tinish belgilari bilan)", "timeout": 30},
    {"name": "kotib-cpu", "port": int(os.environ.get("STT_CPU_PORT", "7009")), "model": None, "timeout": 90},
]
MAX_S, CHUNK_S, BREAK_S = 60, 25, 30
down = {}
locks = {e["port"]: threading.Lock() for e in ENGINES}


def to_wav(audio, max_s):
    p = subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-i", "pipe:0", "-t", str(max_s), "-ac", "1",
                        "-ar", "16000", "-f", "wav", "pipe:1"], input=audio, capture_output=True, timeout=60)
    if p.returncode != 0 or len(p.stdout) < 1000:
        raise ValueError("audio o'qilmadi: " + p.stderr.decode(errors="replace")[:200])
    return p.stdout


def split_wav(wav):
    """16 kHz mono 16-bit: CHUNK_S soniyali bo'laklar (oddiy, sarlavha har biriga qayta yoziladi)."""
    data, step = wav[44:], CHUNK_S * 32000
    out = []
    for i in range(0, len(data), step):
        part = data[i:i + step]
        hdr = bytearray(wav[:44])
        hdr[4:8] = (36 + len(part)).to_bytes(4, "little")
        hdr[40:44] = len(part).to_bytes(4, "little")
        out.append(bytes(hdr) + part)
    return out


def upload(port, wav):
    b = "----stt" + uuid.uuid4().hex
    body = (f"--{b}\r\nContent-Disposition: form-data; name=\"files\"; filename=\"a.wav\"\r\n"
            f"Content-Type: audio/wav\r\n\r\n").encode() + wav + f"\r\n--{b}--\r\n".encode()
    req = urllib.request.Request(f"http://{HOST}:{port}/gradio_api/upload", data=body,
                                 headers={"Content-Type": f"multipart/form-data; boundary={b}"})
    with urllib.request.urlopen(req, timeout=30) as r:
        return json.loads(r.read())[0]


def gradio(port, api, data, timeout):
    base = f"http://{HOST}:{port}/gradio_api"
    req = urllib.request.Request(f"{base}/call/{api}", data=json.dumps({"data": data}).encode(),
                                 headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=15) as r:
        eid = json.loads(r.read())["event_id"]
    event = None
    with urllib.request.urlopen(f"{base}/call/{api}/{eid}", timeout=timeout) as r:
        for raw in r:
            line = raw.decode("utf-8", "replace").rstrip("\n")
            if line.startswith("event:"):
                event = line[6:].strip()
            elif line.startswith("data:") and event in ("complete", "error"):
                if event == "error":
                    raise RuntimeError("gradio: " + line[:200])
                return json.loads(line[5:].strip(), strict=False)
    raise RuntimeError("gradio javob bermadi")


def transcribe(wav):
    seconds = round(max(0, len(wav) - 44) / 32000, 1)
    parts = split_wav(wav) if seconds > CHUNK_S else [wav]
    last = None
    for e in ENGINES:
        if time.time() < down.get(e["name"], 0):
            continue
        try:
            texts = []
            with locks[e["port"]]:                      # bitta model — bir vaqtda bitta so'rov
                for part in parts:
                    path = upload(e["port"], part)
                    args = [{"path": path, "meta": {"_type": "gradio.FileData"}}] + ([e["model"]] if e["model"] else [])
                    r = gradio(e["port"], "transcribe", args, e["timeout"])
                    texts.append(((r[0] if isinstance(r, list) else str(r)) or "").strip())
            return {"text": " ".join(t for t in texts if t).strip(), "engine": e["name"], "audioSeconds": seconds}
        except Exception as ex:  # noqa: BLE001
            down[e["name"]] = time.time() + BREAK_S
            last = ex
    raise RuntimeError(f"STT dvigatellari ishlamayapti: {last}")


class Handler(BaseHTTPRequestHandler):
    def address_string(self):
        return "unix"

    def log_message(self, fmt, *args):
        pass

    def reply(self, code, obj):
        b = json.dumps(obj, ensure_ascii=False).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(b)))
        self.end_headers()
        self.wfile.write(b)

    def do_GET(self):
        if self.path == "/health":
            now = time.time()
            return self.reply(200, {e["name"] + "Down": now < down.get(e["name"], 0) for e in ENGINES})
        self.reply(404, {"error": "not_found"})

    def do_POST(self):
        if self.path != "/stt":
            return self.reply(404, {"error": "not_found"})
        n = int(self.headers.get("Content-Length") or 0)
        if n <= 0 or n > 12 * 1024 * 1024:
            return self.reply(400, {"error": "audio 1 bayt..12 MB"})
        audio = self.rfile.read(n)
        try:
            max_s = min(MAX_S, max(1, int(self.headers.get("X-Max-Seconds") or MAX_S)))
            wav = to_wav(audio, max_s)
        except Exception as ex:  # noqa: BLE001
            return self.reply(400, {"error": str(ex)[:200]})
        if (len(wav) - 44) / 32000 < 0.3:
            return self.reply(400, {"error": "audio juda qisqa"})
        try:
            self.reply(200, transcribe(wav))
        except Exception as ex:  # noqa: BLE001
            self.reply(502, {"error": str(ex)[:300]})


class Server(socketserver.ThreadingMixIn, socketserver.UnixStreamServer):
    daemon_threads = True


if __name__ == "__main__":
    os.makedirs(os.path.dirname(SOCK), exist_ok=True)
    try:
        os.unlink(SOCK)
    except FileNotFoundError:
        pass
    srv = Server(SOCK, Handler)
    os.chmod(SOCK, 0o666)                # API konteyneri nonroot (65532) foydalanuvchida ishlaydi
    print("stt-relay:", SOCK, flush=True)
    srv.serve_forever()
