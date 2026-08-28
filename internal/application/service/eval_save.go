package service

import (
	"encoding/json"
	"fmt"
	"time"

	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/domain/entity"
)

// SaveKRS saves GT for KRS.
func (s *EvalService) SaveKRS(npm string, filename string, req entity.SaveGTRequest) error {
	// Check if GT already exists
	gtExists, _, err := s.store.Exists(npm, "krs", filename)
	if err != nil {
		return apperror.Internal("failed to check gt existence", err)
	}
	if gtExists && !req.ConfirmOverwrite {
		return apperror.Conflict("GT already exists, confirm overwrite")
	}

	// Validate the inner KRS object
	if req.KRS.KRS.Mahasiswa.NPM != npm {
		return apperror.BadRequest("npm mismatch")
	}

	// Validate courses
	if err := validateKRSCourses(req.KRS.KRS.MataKuliah); err != nil {
		return err
	}

	// Set metadata
	req.KRS.Metadata.ExtractedAt = time.Now()
	req.KRS.Metadata.SourceFile = "ground_truth"
	req.KRS.Metadata.DocumentCategory = entity.CategoryGT

	// Marshal and write
	data, err := json.MarshalIndent(req.KRS, "", "  ")
	if err != nil {
		return apperror.Internal("failed to marshal gt", err)
	}

	if err := s.store.WriteGT(npm, "krs", filename, data); err != nil {
		return apperror.Internal("failed to write gt", err)
	}

	return nil
}

// SaveKHS saves GT for KHS.
func (s *EvalService) SaveKHS(npm string, filename string, req entity.SaveGTRequest) error {
	// Check if GT already exists
	gtExists, _, err := s.store.Exists(npm, "khs", filename)
	if err != nil {
		return apperror.Internal("failed to check gt existence", err)
	}
	if gtExists && !req.ConfirmOverwrite {
		return apperror.Conflict("GT already exists, confirm overwrite")
	}

	// Validate the inner KHS object
	if req.KHS.KHS.Mahasiswa.NPM != npm {
		return apperror.BadRequest("npm mismatch")
	}

	// Validate courses
	if err := validateKHSCourses(req.KHS.KHS.MataKuliah); err != nil {
		return err
	}

	// Set metadata
	req.KHS.Metadata.ExtractedAt = time.Now()
	req.KHS.Metadata.SourceFile = "ground_truth"
	req.KHS.Metadata.DocumentCategory = entity.CategoryGT

	// Marshal and write
	data, err := json.MarshalIndent(req.KHS, "", "  ")
	if err != nil {
		return apperror.Internal("failed to marshal gt", err)
	}

	if err := s.store.WriteGT(npm, "khs", filename, data); err != nil {
		return apperror.Internal("failed to write gt", err)
	}

	return nil
}

// validateKRSCourses validates KRS course entries.
func validateKRSCourses(courses []entity.KRSMataKuliah) error {
	noSet := make(map[int]bool)
	for _, c := range courses {
		// Skip completely empty rows
		if c.No == 0 && c.Kode == "" && c.Nama == "" {
			continue
		}
		if c.No <= 0 {
			return apperror.BadRequest("no must be > 0")
		}
		if noSet[c.No] {
			return apperror.BadRequest(fmt.Sprintf("duplicate no: %d", c.No))
		}
		noSet[c.No] = true
		if c.Kode == "" {
			return apperror.BadRequest("kode is required")
		}
	}
	return nil
}

// validateKHSCourses validates KHS course entries.
func validateKHSCourses(courses []entity.KHSMataKuliah) error {
	noSet := make(map[int]bool)
	for _, c := range courses {
		// Skip completely empty rows
		if c.No == 0 && c.Kode == "" && c.Nama == "" {
			continue
		}
		if c.No <= 0 {
			return apperror.BadRequest("no must be > 0")
		}
		if noSet[c.No] {
			return apperror.BadRequest(fmt.Sprintf("duplicate no: %d", c.No))
		}
		noSet[c.No] = true
		if c.Kode == "" {
			return apperror.BadRequest("kode is required")
		}
	}
	return nil
}
