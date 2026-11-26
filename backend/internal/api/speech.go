package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"yoldosh/internal/speech"
)

// POST /speech/stt (multipart: audio, lang?) — ovozni matnga aylantirish.
// O'zbekcha: faqat Kotib (GPU, ishlamasa serverdagi CPU Kotib). Ruscha/inglizcha: Groq Whisper.
// Javob: {text, lang, engine}. Audio saqlanmaydi.
func (s *Server) speechSTT(w http.ResponseWriter, r *http.Request) {
	if s.stt == nil {
		writeError(w, r, http.StatusServiceUnavailable, "stt_unavailable", true)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 11<<20)
	if err := r.ParseMultipartForm(11 << 20); err != nil {
		writeValidation(w, r, fieldErrors{"audio": "multipart, ≤ 10 MB"})
		return
	}
	f, hdr, err := r.FormFile("audio")
	if err != nil {
		writeValidation(w, r, fieldErrors{"audio": "required"})
		return
	}
	audio, err := io.ReadAll(io.LimitReader(f, 10<<20))
	f.Close()
	if err != nil || len(audio) < 1000 {
		writeValidation(w, r, fieldErrors{"audio": "audio juda qisqa yoki bo'sh"})
		return
	}
	lang := strings.ToLower(strings.TrimSpace(r.FormValue("lang")))
	if lang == "" {
		lang = langOf(r)
		if lang != "ru" && lang != "en" {
			lang = "uz"
		}
	}
	if lang != "uz" && lang != "ru" && lang != "en" {
		writeValidation(w, r, fieldErrors{"lang": "uz | ru | en"})
		return
	}
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(2 * time.Minute))
	ctx, cancel := context.WithTimeout(r.Context(), 110*time.Second)
	defer cancel()
	res, err := s.stt.Transcribe(ctx, audio, hdr.Filename, lang)
	switch {
	case errors.Is(err, speech.ErrBadAudio):
		writeValidation(w, r, fieldErrors{"audio": "audio o'qilmadi (webm, ogg, m4a, mp3, wav)"})
		return
	case errors.Is(err, speech.ErrUnavailable):
		slog.Warn("stt: dvigatellar ishlamadi", "err", err)
		writeError(w, r, http.StatusServiceUnavailable, "stt_unavailable", true)
		return
	case err != nil:
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
