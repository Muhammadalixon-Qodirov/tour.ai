package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"yoldosh/internal/speech"
)

func sttPost(t *testing.T, token string, audio []byte, lang string) (int, map[string]any) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if lang != "" {
		mw.WriteField("lang", lang)
	}
	if audio != nil {
		fw, _ := mw.CreateFormFile("audio", "a.m4a")
		fw.Write(audio)
	}
	mw.Close()
	req, _ := http.NewRequest("POST", testSrv.URL+"/api/v1/speech/stt", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestSpeechSTT(t *testing.T) {
	testAPI.limiter = NewLimiter()
	relayUp, localUp := true, true
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !relayUp {
			w.WriteHeader(502)
			return
		}
		if r.ContentLength < 1000 {
			w.WriteHeader(400)
			return
		}
		w.Write([]byte(`{"text":"Registonga qanday boraman?","engine":"kotib-gpu","audioSeconds":2.1}`))
	}))
	defer relay.Close()
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !localUp {
			w.WriteHeader(503)
			return
		}
		w.Write([]byte(`{"text":"Регистонга қандай бораман?","engine":"kotib-local","audioSeconds":2.1}`))
	}))
	defer local.Close()
	var groqCalls int
	groq := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		groqCalls++
		r.ParseMultipartForm(1 << 20)
		if r.FormValue("language") == "uz" {
			t.Error("o'zbekcha uchun Whisper chaqirilmasligi kerak")
		}
		w.Write([]byte(`{"text":"Как добраться до Регистана?"}`))
	}))
	defer groq.Close()
	old := testAPI.stt
	testAPI.stt = speech.New("", "", "gk").WithTestEndpoints(relay.URL, local.URL, groq.URL)
	defer func() { testAPI.stt = old }()

	c := newClient(t, "stt-device-0001")
	audio := bytes.Repeat([]byte{1, 2, 3, 4}, 1000)
	code, out := sttPost(t, c.token, audio, "uz")
	if code != 200 || out["engine"] != "kotib-gpu" || out["text"] != "Registonga qanday boraman?" || out["lang"] != "uz" {
		t.Fatalf("GPU kotib: %d %v", code, out)
	}
	// GPU o'chiq — serverdagi Kotib (kirill bo'lsa lotinga)
	relayUp = false
	code, out = sttPost(t, c.token, audio, "uz")
	if code != 200 || out["engine"] != "kotib-local" || out["text"] != "Registonga qanday boraman?" {
		t.Fatalf("serverdagi kotib: %d %v", code, out)
	}
	// ikkala Kotib ham ishlamasa — 503 (Whisper ga o'tmaydi)
	localUp = false
	if code, out := sttPost(t, c.token, audio, "uz"); code != 503 {
		t.Fatalf("kotibsiz: %d %v", code, out)
	}
	// ruscha — Whisper
	code, out = sttPost(t, c.token, audio, "ru")
	if code != 200 || out["engine"] != "whisper" || out["text"] != "Как добраться до Регистана?" {
		t.Fatalf("ruscha: %d %v", code, out)
	}
	if code, _ := sttPost(t, c.token, nil, "uz"); code != 400 {
		t.Fatalf("audiosiz: %d", code)
	}
	if code, _ := sttPost(t, c.token, audio, "de"); code != 400 {
		t.Fatalf("til: %d", code)
	}
	if groqCalls != 1 {
		t.Fatalf("groq chaqiruvlari: %d (faqat ruscha bitta)", groqCalls)
	}
}
