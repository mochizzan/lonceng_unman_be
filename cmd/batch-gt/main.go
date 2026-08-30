package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lonceng_unman_be/internal/domain/entity"
)

// gtStats tracks the results of batch ground truth creation.
type gtStats struct {
	Total        int
	Created      int
	Skipped      int
	Failed       int
	Errors       []string
	CreatedFiles []string
}

func main() {
	var (
		extractDir = flag.String("extract-dir", "./extracted", "Path to extracted JSON directory")
		evalDir    = flag.String("eval-dir", "./eval/ground_truth", "Path to ground truth output directory")
		npmFilter  = flag.String("npm", "", "Only create GT for specific NPM (empty = all)")
		docType    = flag.String("type", "all", "Document type: krs, khs, or all")
		force      = flag.Bool("force", false, "Overwrite existing ground truth")
		dryRun     = flag.Bool("dry-run", false, "Show what would be created without actually doing it")
		verbose    = flag.Bool("v", false, "Verbose output")
	)
	flag.Parse()

	// Validate doc type
	docTypeLower := strings.ToLower(*docType)
	if docTypeLower != "all" && docTypeLower != "krs" && docTypeLower != "khs" {
		fmt.Fprintf(os.Stderr, "Error: invalid doc type %q, must be krs, khs, or all\n", *docType)
		os.Exit(1)
	}

	// Resolve absolute paths
	absExtract, err := filepath.Abs(*extractDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving extract dir: %v\n", err)
		os.Exit(1)
	}
	absEval, err := filepath.Abs(*evalDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving eval dir: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("============================================================")
	fmt.Println("BATCH GROUND TRUTH CREATION")
	fmt.Println("============================================================")
	fmt.Printf("Extract dir: %s\n", absExtract)
	fmt.Printf("Eval dir:    %s\n", absEval)
	fmt.Printf("NPM filter:  %s\n", ternary(*npmFilter == "", "(all)", *npmFilter))
	fmt.Printf("Doc type:    %s\n", docTypeLower)
	fmt.Printf("Force:       %v\n", *force)
	fmt.Printf("Dry run:     %v\n", *dryRun)
	fmt.Println("============================================================")

	stats := &gtStats{
		Errors:       make([]string, 0),
		CreatedFiles: make([]string, 0),
	}

	startTime := time.Now()

	// Process KRS if requested
	if docTypeLower == "all" || docTypeLower == "krs" {
		createGTKRS(absExtract, absEval, *npmFilter, *force, *dryRun, *verbose, stats)
	}

	// Process KHS if requested
	if docTypeLower == "all" || docTypeLower == "khs" {
		createGTKHS(absExtract, absEval, *npmFilter, *force, *dryRun, *verbose, stats)
	}

	elapsed := time.Since(startTime)

	// Print summary
	fmt.Println()
	fmt.Println("============================================================")
	fmt.Println("SUMMARY")
	fmt.Println("============================================================")
	fmt.Printf("Total extractions: %d\n", stats.Total)
	fmt.Printf("GT created:        %d\n", stats.Created)
	fmt.Printf("GT skipped:        %d\n", stats.Skipped)
	fmt.Printf("GT failed:         %d\n", stats.Failed)
	fmt.Printf("Elapsed:           %v\n", elapsed)

	if len(stats.Errors) > 0 {
		fmt.Println()
		fmt.Println("ERRORS:")
		for _, e := range stats.Errors {
			fmt.Printf("  - %s\n", e)
		}
	}

	if *dryRun {
		fmt.Println()
		fmt.Println("DRY RUN - no files were written")
	}
}

// createGTKRS creates ground truth files for KRS documents.
func createGTKRS(extractDir, evalDir, npmFilter string, force, dryRun, verbose bool, stats *gtStats) {
	// Find all KRS JSONs
	krsPattern := filepath.Join(extractDir, "*", "krs", "*.json")
	jsons, err := filepath.Glob(krsPattern)
	if err != nil {
		stats.Errors = append(stats.Errors, fmt.Sprintf("glob krs: %v", err))
		return
	}

	sort.Strings(jsons)

	for _, jsonPath := range jsons {
		// Extract NPM from path: extracted/{NPM}/krs/semester_X.json
		rel, err := filepath.Rel(extractDir, jsonPath)
		if err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("rel path %s: %v", jsonPath, err))
			continue
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) < 3 {
			stats.Errors = append(stats.Errors, fmt.Sprintf("unexpected path structure: %s", jsonPath))
			continue
		}
		npm := parts[0]

		// Apply NPM filter
		if npmFilter != "" && npm != npmFilter {
			continue
		}

		stats.Total++

		filename := filepath.Base(jsonPath)
		gtPath := filepath.Join(evalDir, npm, "krs", filename)

		// Check if GT already exists
		if !force {
			if _, err := os.Stat(gtPath); err == nil {
				if verbose {
					fmt.Printf("  [SKIP] GT KRS %s/%s (already exists)\n", npm, filename)
				}
				stats.Skipped++
				continue
			}
		}

		// Read the extraction
		data, err := os.ReadFile(jsonPath)
		if err != nil {
			stats.Failed++
			stats.Errors = append(stats.Errors, fmt.Sprintf("read %s: %v", jsonPath, err))
			continue
		}

		// Parse the extraction
		var extraction entity.KRSExtraction
		if err := json.Unmarshal(data, &extraction); err != nil {
			stats.Failed++
			stats.Errors = append(stats.Errors, fmt.Sprintf("unmarshal %s: %v", jsonPath, err))
			continue
		}

		// Convert to ground truth
		gt := convertToGTKRS(&extraction, npm)

		// Marshal the GT
		gtData, err := json.MarshalIndent(gt, "", "  ")
		if err != nil {
			stats.Failed++
			stats.Errors = append(stats.Errors, fmt.Sprintf("marshal gt %s: %v", jsonPath, err))
			continue
		}

		if dryRun {
			fmt.Printf("  [DRY]  GT KRS %s/%s → %s (%d courses)\n", npm, filename, gtPath, len(gt.KRS.MataKuliah))
			stats.Created++
			stats.CreatedFiles = append(stats.CreatedFiles, fmt.Sprintf("krs/%s/%s", npm, filename))
			continue
		}

		// Create output directory
		gtDir := filepath.Dir(gtPath)
		if err := os.MkdirAll(gtDir, 0o755); err != nil {
			stats.Failed++
			stats.Errors = append(stats.Errors, fmt.Sprintf("mkdir %s: %v", gtDir, err))
			continue
		}

		// Write atomically (tmp + rename)
		tmpPath := gtPath + ".tmp"
		if err := os.WriteFile(tmpPath, gtData, 0o644); err != nil {
			os.Remove(tmpPath)
			stats.Failed++
			stats.Errors = append(stats.Errors, fmt.Sprintf("write %s: %v", tmpPath, err))
			continue
		}
		if err := os.Rename(tmpPath, gtPath); err != nil {
			os.Remove(tmpPath)
			stats.Failed++
			stats.Errors = append(stats.Errors, fmt.Sprintf("rename %s: %v", gtPath, err))
			continue
		}

		stats.Created++
		stats.CreatedFiles = append(stats.CreatedFiles, fmt.Sprintf("krs/%s/%s", npm, filename))
		fmt.Printf("  [OK]   GT KRS %s/%s (%d courses)\n", npm, filename, len(gt.KRS.MataKuliah))
	}
}

// createGTKHS creates ground truth files for KHS documents.
func createGTKHS(extractDir, evalDir, npmFilter string, force, dryRun, verbose bool, stats *gtStats) {
	// Find all KHS JSONs
	khsPattern := filepath.Join(extractDir, "*", "khs", "*.json")
	jsons, err := filepath.Glob(khsPattern)
	if err != nil {
		stats.Errors = append(stats.Errors, fmt.Sprintf("glob khs: %v", err))
		return
	}

	sort.Strings(jsons)

	for _, jsonPath := range jsons {
		// Extract NPM from path: extracted/{NPM}/khs/2022_2023_GANJIL.json
		rel, err := filepath.Rel(extractDir, jsonPath)
		if err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("rel path %s: %v", jsonPath, err))
			continue
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) < 3 {
			stats.Errors = append(stats.Errors, fmt.Sprintf("unexpected path structure: %s", jsonPath))
			continue
		}
		npm := parts[0]

		// Apply NPM filter
		if npmFilter != "" && npm != npmFilter {
			continue
		}

		stats.Total++

		filename := filepath.Base(jsonPath)
		gtPath := filepath.Join(evalDir, npm, "khs", filename)

		// Check if GT already exists
		if !force {
			if _, err := os.Stat(gtPath); err == nil {
				if verbose {
					fmt.Printf("  [SKIP] GT KHS %s/%s (already exists)\n", npm, filename)
				}
				stats.Skipped++
				continue
			}
		}

		// Read the extraction
		data, err := os.ReadFile(jsonPath)
		if err != nil {
			stats.Failed++
			stats.Errors = append(stats.Errors, fmt.Sprintf("read %s: %v", jsonPath, err))
			continue
		}

		// Parse the extraction
		var extraction entity.KHSExtraction
		if err := json.Unmarshal(data, &extraction); err != nil {
			stats.Failed++
			stats.Errors = append(stats.Errors, fmt.Sprintf("unmarshal %s: %v", jsonPath, err))
			continue
		}

		// Convert to ground truth
		gt := convertToGTKHS(&extraction, npm)

		// Marshal the GT
		gtData, err := json.MarshalIndent(gt, "", "  ")
		if err != nil {
			stats.Failed++
			stats.Errors = append(stats.Errors, fmt.Sprintf("marshal gt %s: %v", jsonPath, err))
			continue
		}

		if dryRun {
			fmt.Printf("  [DRY]  GT KHS %s/%s → %s (%d courses)\n", npm, filename, gtPath, len(gt.KHS.MataKuliah))
			stats.Created++
			stats.CreatedFiles = append(stats.CreatedFiles, fmt.Sprintf("khs/%s/%s", npm, filename))
			continue
		}

		// Create output directory
		gtDir := filepath.Dir(gtPath)
		if err := os.MkdirAll(gtDir, 0o755); err != nil {
			stats.Failed++
			stats.Errors = append(stats.Errors, fmt.Sprintf("mkdir %s: %v", gtDir, err))
			continue
		}

		// Write atomically (tmp + rename)
		tmpPath := gtPath + ".tmp"
		if err := os.WriteFile(tmpPath, gtData, 0o644); err != nil {
			os.Remove(tmpPath)
			stats.Failed++
			stats.Errors = append(stats.Errors, fmt.Sprintf("write %s: %v", tmpPath, err))
			continue
		}
		if err := os.Rename(tmpPath, gtPath); err != nil {
			os.Remove(tmpPath)
			stats.Failed++
			stats.Errors = append(stats.Errors, fmt.Sprintf("rename %s: %v", gtPath, err))
			continue
		}

		stats.Created++
		stats.CreatedFiles = append(stats.CreatedFiles, fmt.Sprintf("khs/%s/%s", npm, filename))
		fmt.Printf("  [OK]   GT KHS %s/%s (%d courses)\n", npm, filename, len(gt.KHS.MataKuliah))
	}
}

// convertToGTKRS converts a KRS extraction result to a ground truth structure.
func convertToGTKRS(extraction *entity.KRSExtraction, npm string) entity.KRSExtraction {
	gt := *extraction

	// Ensure NPM matches
	gt.KRS.Mahasiswa.NPM = npm

	// Set metadata
	gt.Metadata.ExtractedAt = time.Now()
	gt.Metadata.SourceFile = "ground_truth"
	gt.Metadata.DocumentCategory = entity.CategoryGT

	return gt
}

// convertToGTKHS converts a KHS extraction result to a ground truth structure.
func convertToGTKHS(extraction *entity.KHSExtraction, npm string) entity.KHSExtraction {
	gt := *extraction

	// Ensure NPM matches
	gt.KHS.Mahasiswa.NPM = npm

	// Set metadata
	gt.Metadata.ExtractedAt = time.Now()
	gt.Metadata.SourceFile = "ground_truth"
	gt.Metadata.DocumentCategory = entity.CategoryGT

	return gt
}

// ternary is a simple inline ternary helper.
func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
