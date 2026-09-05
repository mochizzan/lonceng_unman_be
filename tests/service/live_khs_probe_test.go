package service_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// liveProbe hits the running server (127.0.0.1:3000) with the provided NPM.
// Marked with t.Skip unless LIVE_PROBE=1. Used during diagnosis, not in CI.
// Credentials are read from LIVE_NPM / LIVE_PASSWORD env (fallback to
// 2211700006 / Izzan027 supplied by the user in this session).
func liveProbe(t *testing.T, npm, password string) []byte {
	t.Helper()
	if os.Getenv("LIVE_PROBE") != "1" {
		t.Skip("set LIVE_PROBE=1 to run live LMS probe")
	}
	body, _ := json.Marshal(map[string]string{"npm": npm, "password": password})
	req, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:3000/api/v1/lms/khs/semesters", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("live probe request failed: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	t.Logf("live probe status=%d body=%.800s", resp.StatusCode, string(b))
	return b
}

func TestLive_KHSProbe_GENAP_2023_2024(t *testing.T) {
	npm := os.Getenv("LIVE_NPM")
	if npm == "" {
		npm = "2211700006"
	}
	pw := os.Getenv("LIVE_PASSWORD")
	if pw == "" {
		pw = "Izzan027"
	}
	b := liveProbe(t, npm, pw)
	_ = b
	_ = fmt.Sprintf("ok")
}

func TestLive_KHSSingle_GANJIL_2023_2024(t *testing.T) {
	npm := os.Getenv("LIVE_NPM")
	if npm == "" {
		npm = "2211700006"
	}
	pw := os.Getenv("LIVE_PASSWORD")
	if pw == "" {
		pw = "Izzan027"
	}
	if os.Getenv("LIVE_PROBE") != "1" {
		t.Skip("set LIVE_PROBE=1")
	}
	body, _ := json.Marshal(map[string]string{"npm": npm, "password": pw, "tahun_ajaran": "2023/2024", "semester": "GANJIL"})
	req, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:3000/api/v1/lms/khs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	t.Logf("status=%d body=%.1000s", resp.StatusCode, string(b))
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 for GANJIL 2023/2024, got %d", resp.StatusCode)
	}
}

func TestLive_KHSSingle_GENAP_2023_2024(t *testing.T) {
	npm := os.Getenv("LIVE_NPM")
	if npm == "" {
		npm = "2211700006"
	}
	pw := os.Getenv("LIVE_PASSWORD")
	if pw == "" {
		pw = "Izzan027"
	}
	if os.Getenv("LIVE_PROBE") != "1" {
		t.Skip("set LIVE_PROBE=1")
	}
	body, _ := json.Marshal(map[string]string{"npm": npm, "password": pw, "tahun_ajaran": "2023/2024", "semester": "GENAP"})
	req, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:3000/api/v1/lms/khs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	t.Logf("status=%d body=%.1200s", resp.StatusCode, string(b))
	// This is the semester that 503'd in paste-1.md — assert it now returns 200 or a 503 with DIAG-NAV logs.
	if resp.StatusCode == 503 {
		t.Logf("REPRO CONFIRMED: GENAP 2023/2024 still 503 — see server DIAG-NAV logs for H1/H2/H4 triage")
	} else if resp.StatusCode != 200 {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}
}
