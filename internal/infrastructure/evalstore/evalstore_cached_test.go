package evalstore_test

import (
	"path/filepath"
	"testing"

	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/infrastructure/evalstore"
)

// mockStore is a test double that records call counts.
type mockStore struct {
	loadGTCalls      int
	loadExtractCalls int
	gtData           map[string][]byte
	extractData      map[string][]byte
}

func newMockStore() *mockStore {
	return &mockStore{
		gtData:      make(map[string][]byte),
		extractData: make(map[string][]byte),
	}
}

func (m *mockStore) ListNPMs() ([]string, error) { return nil, nil }
func (m *mockStore) ListDocs(npm string) ([]entity.NPMDoc, error) {
	return nil, nil
}

func (m *mockStore) LoadGT(npm, docType, filename string) ([]byte, error) {
	m.loadGTCalls++
	key := npm + "/" + docType + "/" + filename
	return m.gtData[key], nil
}

func (m *mockStore) LoadExtract(npm, docType, filename string) ([]byte, error) {
	m.loadExtractCalls++
	key := npm + "/" + docType + "/" + filename
	return m.extractData[key], nil
}

func (m *mockStore) WriteGT(npm, docType, filename string, data []byte) error {
	return nil
}

func (m *mockStore) Exists(npm, docType, filename string) (bool, bool, error) {
	return false, false, nil
}

func TestCachedStoreLoadGT_CacheHit(t *testing.T) {
	mock := newMockStore()
	mock.gtData["2211700006/krs/semester_8.json"] = []byte(`{"krs":{"mahasiswa":{"nama":"John Doe"}}}`)

	cs := evalstore.NewCachedStore(mock)

	// First call — cache miss, reads from inner store
	data1, err := cs.LoadGT("2211700006", "krs", "semester_8.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data1) != `{"krs":{"mahasiswa":{"nama":"John Doe"}}}` {
		t.Fatalf("unexpected data: %s", data1)
	}
	if mock.loadGTCalls != 1 {
		t.Fatalf("expected 1 LoadGT call, got %d", mock.loadGTCalls)
	}

	// Second call — cache hit, no inner store call
	data2, err := cs.LoadGT("2211700006", "krs", "semester_8.json")
	if err != nil {
		t.Fatalf("unexpected error on cache hit: %v", err)
	}
	if string(data2) != string(data1) {
		t.Fatalf("cache hit returned different data: %s vs %s", data2, data1)
	}
	if mock.loadGTCalls != 1 {
		t.Fatalf("expected still 1 LoadGT call after cache hit, got %d", mock.loadGTCalls)
	}
}

func TestCachedStoreLoadExtract_CacheHit(t *testing.T) {
	mock := newMockStore()
	mock.extractData["2211700006/khs/2022_2023_GANJIL.json"] = []byte(`{"khs":{"mahasiswa":{"nama":"Jane Smith"}}}`)

	cs := evalstore.NewCachedStore(mock)

	data1, err := cs.LoadExtract("2211700006", "khs", "2022_2023_GANJIL.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data1) != `{"khs":{"mahasiswa":{"nama":"Jane Smith"}}}` {
		t.Fatalf("unexpected data: %s", data1)
	}
	if mock.loadExtractCalls != 1 {
		t.Fatalf("expected 1 LoadExtract call, got %d", mock.loadExtractCalls)
	}

	// Cache hit
	data2, err := cs.LoadExtract("2211700006", "khs", "2022_2023_GANJIL.json")
	if err != nil {
		t.Fatalf("unexpected error on cache hit: %v", err)
	}
	if string(data2) != string(data1) {
		t.Fatalf("cache hit returned different data: %s vs %s", data2, data1)
	}
	if mock.loadExtractCalls != 1 {
		t.Fatalf("expected still 1 LoadExtract call after cache hit, got %d", mock.loadExtractCalls)
	}
}

func TestCachedStoreMiss_DifferentKeys(t *testing.T) {
	mock := newMockStore()
	mock.gtData["2211700006/krs/semester_8.json"] = []byte(`{"file":"A"}`)
	mock.gtData["2211700006/krs/semester_9.json"] = []byte(`{"file":"B"}`)

	cs := evalstore.NewCachedStore(mock)

	dataA, err := cs.LoadGT("2211700006", "krs", "semester_8.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dataB, err := cs.LoadGT("2211700006", "krs", "semester_9.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(dataA) == string(dataB) {
		t.Fatalf("different keys should return different data")
	}
	if string(dataA) != `{"file":"A"}` {
		t.Fatalf("unexpected dataA: %s", dataA)
	}
	if string(dataB) != `{"file":"B"}` {
		t.Fatalf("unexpected dataB: %s", dataB)
	}
	if mock.loadGTCalls != 2 {
		t.Fatalf("expected 2 LoadGT calls for different keys, got %d", mock.loadGTCalls)
	}
}

func TestCachedStorePassthrough(t *testing.T) {
	// Test that ListNPMs, ListDocs, WriteGT, Exists, LoadGT passthrough works
	tmpDir := t.TempDir()
	store := evalstore.New(filepath.Join(tmpDir, "eval"), filepath.Join(tmpDir, "extract"), filepath.Join(tmpDir, "download"))

	cs := evalstore.NewCachedStore(store)

	// ListNPMs should return empty (no dirs yet)
	npms, err := cs.ListNPMs()
	if err != nil {
		t.Fatalf("ListNPMs error: %v", err)
	}
	if len(npms) != 0 {
		t.Fatalf("expected 0 NPMs, got %d", len(npms))
	}

	// ListDocs should return empty
	docs, err := cs.ListDocs("2211700006")
	if err != nil {
		t.Fatalf("ListDocs error: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected 0 docs, got %d", len(docs))
	}

	// WriteGT should succeed
	err = cs.WriteGT("2211700006", "krs", "semester_8.json", []byte(`{"test":true}`))
	if err != nil {
		t.Fatalf("WriteGT error: %v", err)
	}

	// LoadGT should read the written file
	data, err := cs.LoadGT("2211700006", "krs", "semester_8.json")
	if err != nil {
		t.Fatalf("LoadGT error: %v", err)
	}
	if string(data) != `{"test":true}` {
		t.Fatalf("unexpected data: %s", data)
	}

	// Exists should return true for GT
	hasGT, hasExtract, err := cs.Exists("2211700006", "krs", "semester_8.json")
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if !hasGT {
		t.Fatalf("expected hasGT=true")
	}
	if hasExtract {
		t.Fatalf("expected hasExtract=false")
	}
}
