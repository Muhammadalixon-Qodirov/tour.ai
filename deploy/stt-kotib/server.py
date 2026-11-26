"""stt-kotib — Kotib (o'zbekcha STT) HTTP xizmati, CPU, faster-whisper.

POST /transcribe  body: istalgan audio (webm/ogg/m4a/mp3/wav), ixtiyoriy ?lang=uz
                  -> {"text": "...", "engine": "kotib-local", "audioSeconds": 3.2, "ms": 2100}
GET  /health      -> {"ok": true}
Bir vaqtda bitta so'rov (umumiy server CPU sini to'ldirmaslik uchun), audio diskka yozilmaydi.
"""
import json, os, subprocess, threading, time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import numpy as np
from faster_whisper import WhisperModel

THREADS = int(os.environ.get("KOTIB_THREADS", "4"))
MAX_S = 60
model = WhisperModel("/model", device="cpu", compute_type="int8", cpu_threads=THREADS)
lock = threading.Lock()


def decode(audio):
    p = subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-i", "pipe:0", "-t", str(MAX_S), "-ac", "1",
                        "-ar", "16000", "-f", "s16le", "pipe:1"], input=audio, capture_output=True, timeout=60)
    if p.returncode != 0 or len(p.stdout) < 3200:
        raise ValueError("audio o'qilmadi: " + p.stderr.decode(errors="replace")[:160])
    return np.frombuffer(p.stdout, np.int16).astype(np.float32) / 32768.0


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
        if not self.path.startswith("/transcribe"):
            return self.reply(404, {"error": "not_found"})
        n = int(self.headers.get("Content-Length") or 0)
        if n <= 0 or n > 12 * 1024 * 1024:
            return self.reply(400, {"error": "audio 1 bayt..12 MB"})
        try:
            pcm = decode(self.rfile.read(n))
        except Exception as e:  # noqa: BLE001
            return self.reply(400, {"error": str(e)})
        secs = round(len(pcm) / 16000, 1)
        if secs < 0.3:
            return self.reply(400, {"error": "audio juda qisqa"})
        t = time.time()
        with lock:
            segs, _ = model.transcribe(pcm, language="uz", beam_size=5, vad_filter=True,
                                       condition_on_previous_text=False)
            text = " ".join(s.text.strip() for s in segs).strip()
        self.reply(200, {"text": text, "engine": "kotib-local", "audioSeconds": secs, "ms": round((time.time() - t) * 1000)})


if __name__ == "__main__":
    print("stt-kotib: model yuklandi, :8000", flush=True)
    ThreadingHTTPServer(("0.0.0.0", 8000), H).serve_forever()
