package port

import "lonceng_unman_be/internal/domain/entity"

// EvalStore manages ground-truth JSON files and reads the extract cache.
type EvalStore interface {
	// ListNPMs returns all NPMs that have either extract or GT.
	ListNPMs() ([]string, error)
	// ListDocs returns all document files for a NPM (krs/khs).
	ListDocs(npm string) ([]entity.NPMDoc, error)
	// LoadGT loads GT JSON for the given doc type and filename.
	LoadGT(npm string, docType string, filename string) ([]byte, error)
	// LoadExtract loads extract JSON for the given doc type and filename.
	LoadExtract(npm string, docType string, filename string) ([]byte, error)
	// WriteGT writes GT JSON (atomic).
	WriteGT(npm string, docType string, filename string, data []byte) error
	// Exists checks if GT or extract exists for the pair.
	Exists(npm string, docType string, filename string) (bool, bool, error)
}

// EvalService provides business logic for evaluation.
type EvalService interface {
	// Index returns aggregate view for /eval page.
	Index() (entity.IndexEval, error)
	// Student returns full evaluation for one NPM.
	Student(npm string) (entity.StudentEval, error)
	// SaveKRS saves GT for KRS (write-only POST).
	SaveKRS(npm string, filename string, req entity.SaveGTRequest) error
	// SaveKHS saves GT for KHS.
	SaveKHS(npm string, filename string, req entity.SaveGTRequest) error
}
