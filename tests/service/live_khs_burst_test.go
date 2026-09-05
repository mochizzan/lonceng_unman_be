package service_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestLive_KHS_Burst8_Sequential(t *testing.T) {
	if os.Getenv("LIVE_PROBE") != "1" {
		t.Skip("set LIVE_PROBE=1")
	}
	npm := os.Getenv("LIVE_NPM")
	if npm == "" {
		npm = "2211700006"
	}
	pw := os.Getenv("LIVE_PASSWORD")
	if pw == "" {
		pw = "Izzan027"
	}
	cases := []struct{ ta, sem string }{
		{"2022/2023", "GANJIL"},
		{"2022/2023", "GENAP"},
		{"2023/2024", "GANJIL"},
		{"2023/2024", "GENAP"}, // paste-1 victim
		{"2024/2025", "GANJIL"},
		{"2024/2025", "GENAP"},
		{"2025/2026", "GANJIL"},
		{"2025/2026", "GENAP"},
	}
	client := &http.Client{Timeout: 90 * time.Second}
	for i, c := range cases {
		body, _ := json.Marshal(map[string]string{"npm": npm, "password": pw, "tahun_ajaran": c.ta, "semester": c.sem})
		req, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:3000/api/v1/lms/khs", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		start := time.Now()
		resp, err := client.Do(req)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("[%d] %s %s request failed after %v: %v", i, c.ta, c.sem, elapsed, err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Logf("[%d] %s %s → HTTP %d in %v body=%.600s", i, c.ta, c.sem, resp.StatusCode, elapsed, string(b))
		if resp.StatusCode != 200 {
			t.Errorf("[%d] %s %s expected 200 got %d", i, c.ta, c.sem, resp.StatusCode)
		}
		// No sleep — replicate paste-1 burst timing that triggered 503.
	}
}
