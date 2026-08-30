package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lonceng_unman_be/internal/infrastructure/extractor"
)

// extractionStats tracks the results of batch extraction.
type extractionStats struct {
	TotalPDFs    int
	SuccessKRS   int
	SuccessKHS   int
	SkippedKRS   int
	SkippedKHS   int
	FailedKRS    int
	FailedKHS    int
	Errors       []string
	ExtractedKRS []string
	ExtractedKHS []string
}

func main() {
	var (
		downloadDir = flag.String("download-dir", "./downloads", "Path to downloads directory")
		extractDir  = flag.String("extract-dir", "./extracted", "Path to extracted output directory")
		npmFilter   = flag.String("npm", "", "Only extract for specific NPM (empty = all)")
		docType     = flag.String("type", "all", "Document type: krs, khs, or all")
		force       = flag.Bool("force", false, "Overwrite existing extractions")
		dryRun      = flag.Bool("dry-run", false, "Show what would be extracted without actually doing it")
		verbose     = flag.Bool("v", false, "Verbose output")
	)
	flag.Parse()

	// Validate doc type
	docTypeLower := strings.ToLower(*docType)
	if docTypeLower != "all" && docTypeLower != "krs" && docTypeLower != "khs" {
		fmt.Fprintf(os.Stderr, "Error: invalid doc type %q, must be krs, khs, or all\n", *docType)
		os.Exit(1)
	}

	// Resolve absolute paths
	absDownload, err := filepath.Abs(*downloadDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving download dir: %v\n", err)
		os.Exit(1)
	}
	absExtract, err := filepath.Abs(*extractDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving extract dir: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("============================================================")
	fmt.Println("BATCH PDF EXTRACTION")
	fmt.Println("============================================================")
	fmt.Printf("Download dir: %s\n", absDownload)
	fmt.Printf("Extract dir:  %s\n", absExtract)
	fmt.Printf("NPM filter:   %s\n", ternary(*npmFilter == "", "(all)", *npmFilter))
	fmt.Printf("Doc type:     %s\n", docTypeLower)
	fmt.Printf("Force:        %v\n", *force)
	fmt.Printf("Dry run:      %v\n", *dryRun)
	fmt.Println("============================================================")

	stats := &extractionStats{
		Errors:       make([]string, 0),
		ExtractedKRS: make([]string, 0),
		ExtractedKHS: make([]string, 0),
	}

	startTime := time.Now()

	// Process KRS if requested
	if docTypeLower == "all" || docTypeLower == "krs" {
		processKRS(absDownload, absExtract, *npmFilter, *force, *dryRun, *verbose, stats)
	}

	// Process KHS if requested
	if docTypeLower == "all" || docTypeLower == "khs" {
		processKHS(absDownload, absExtract, *npmFilter, *force, *dryRun, *verbose, stats)
	}

	elapsed := time.Since(startTime)

	// Print summary
	fmt.Println()
	fmt.Println("============================================================")
	fmt.Println("SUMMARY")
	fmt.Println("============================================================")
	fmt.Printf("Total PDFs found:    %d\n", stats.TotalPDFs)
	fmt.Printf("KRS extracted:       %d\n", stats.SuccessKRS)
	fmt.Printf("KRS skipped:         %d\n", stats.SkippedKRS)
	fmt.Printf("KRS failed:          %d\n", stats.FailedKRS)
	fmt.Printf("KHS extracted:       %d\n", stats.SuccessKHS)
	fmt.Printf("KHS skipped:         %d\n", stats.SkippedKHS)
	fmt.Printf("KHS failed:          %d\n", stats.FailedKHS)
	fmt.Printf("Elapsed:             %v\n", elapsed)

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

// processKRS extracts all KRS PDFs from the download directory.
func processKRS(downloadDir, extractDir, npmFilter string, force, dryRun, verbose bool, stats *extractionStats) {
	// Find all KRS PDFs
	krsPattern := filepath.Join(downloadDir, "*", "krs", "*.pdf")
	pdfs, err := filepath.Glob(krsPattern)
	if err != nil {
		stats.Errors = append(stats.Errors, fmt.Sprintf("glob krs: %v", err))
		return
	}

	// Sort for deterministic order
	sort.Strings(pdfs)

	for _, pdfPath := range pdfs {
		// Extract NPM from path: downloads/{NPM}/krs/semester_X.pdf
		rel, err := filepath.Rel(downloadDir, pdfPath)
		if err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("rel path %s: %v", pdfPath, err))
			continue
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) < 3 {
			stats.Errors = append(stats.Errors, fmt.Sprintf("unexpected path structure: %s", pdfPath))
			continue
		}
		npm := parts[0]

		// Apply NPM filter
		if npmFilter != "" && npm != npmFilter {
			continue
		}

		stats.TotalPDFs++

		// Determine output filename
		baseName := strings.TrimSuffix(filepath.Base(pdfPath), ".pdf")
		outputFile := baseName + ".json"
		outputPath := filepath.Join(extractDir, npm, "krs", outputFile)

		// Check if already exists
		if !force {
			if _, err := os.Stat(outputPath); err == nil {
				if verbose {
					fmt.Printf("  [SKIP] KRS %s/%s (already exists)\n", npm, outputFile)
				}
				stats.SkippedKRS++
				continue
			}
		}

		if dryRun {
			fmt.Printf("  [DRY]  KRS %s/%s → %s\n", npm, filepath.Base(pdfPath), outputPath)
			stats.SuccessKRS++
			stats.ExtractedKRS = append(stats.ExtractedKRS, fmt.Sprintf("%s/%s", npm, outputFile))
			continue
		}

		// Parse the PDF
		if verbose {
			fmt.Printf("  [EXTRACT] KRS %s/%s ...\n", npm, filepath.Base(pdfPath))
		}

		result, err := extractor.ParseKRS(pdfPath, npm)
		if err != nil {
			stats.FailedKRS++
			stats.Errors = append(stats.Errors, fmt.Sprintf("parse krs %s: %v", pdfPath, err))
			fmt.Printf("  [FAIL] KRS %s/%s: %v\n", npm, filepath.Base(pdfPath), err)
			continue
		}

		// Marshal to JSON
		data, err := extractor.MarshalJSON(result)
		if err != nil {
			stats.FailedKRS++
			stats.Errors = append(stats.Errors, fmt.Sprintf("marshal krs %s: %v", pdfPath, err))
			continue
		}

		// Create output directory
		outputDir := filepath.Dir(outputPath)
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			stats.FailedKRS++
			stats.Errors = append(stats.Errors, fmt.Sprintf("mkdir %s: %v", outputDir, err))
			continue
		}

		// Write atomically (tmp + rename)
		tmpPath := outputPath + ".tmp"
		if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
			os.Remove(tmpPath)
			stats.FailedKRS++
			stats.Errors = append(stats.Errors, fmt.Sprintf("write %s: %v", tmpPath, err))
			continue
		}
		if err := os.Rename(tmpPath, outputPath); err != nil {
			os.Remove(tmpPath)
			stats.FailedKRS++
			stats.Errors = append(stats.Errors, fmt.Sprintf("rename %s: %v", outputPath, err))
			continue
		}

		stats.SuccessKRS++
		stats.ExtractedKRS = append(stats.ExtractedKRS, fmt.Sprintf("%s/%s", npm, outputFile))
		fmt.Printf("  [OK]   KRS %s/%s → %s (%d courses)\n", npm, filepath.Base(pdfPath), outputFile, len(result.KRS.MataKuliah))
	}
}

// processKHS extracts all KHS PDFs from the download directory.
func processKHS(downloadDir, extractDir, npmFilter string, force, dryRun, verbose bool, stats *extractionStats) {
	// Find all KHS PDFs
	khsPattern := filepath.Join(downloadDir, "*", "khs", "*.pdf")
	pdfs, err := filepath.Glob(khsPattern)
	if err != nil {
		stats.Errors = append(stats.Errors, fmt.Sprintf("glob khs: %v", err))
		return
	}

	// Sort for deterministic order
	sort.Strings(pdfs)

	for _, pdfPath := range pdfs {
		// Extract NPM from path: downloads/{NPM}/khs/2022_2023_GANJIL.pdf
		rel, err := filepath.Rel(downloadDir, pdfPath)
		if err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("rel path %s: %v", pdfPath, err))
			continue
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) < 3 {
			stats.Errors = append(stats.Errors, fmt.Sprintf("unexpected path structure: %s", pdfPath))
			continue
		}
		npm := parts[0]

		// Apply NPM filter
		if npmFilter != "" && npm != npmFilter {
			continue
		}

		stats.TotalPDFs++

		// Parse filename to get tahun ajaran and semester
		baseName := strings.TrimSuffix(filepath.Base(pdfPath), ".pdf")
		tahunAjaran, semester, err := parseKHSFilename(baseName)
		if err != nil {
			stats.FailedKHS++
			stats.Errors = append(stats.Errors, fmt.Sprintf("parse filename %s: %v", pdfPath, err))
			continue
		}

		// Determine output filename
		outputFile := baseName + ".json"
		outputPath := filepath.Join(extractDir, npm, "khs", outputFile)

		// Check if already exists
		if !force {
			if _, err := os.Stat(outputPath); err == nil {
				if verbose {
					fmt.Printf("  [SKIP] KHS %s/%s (already exists)\n", npm, outputFile)
				}
				stats.SkippedKHS++
				continue
			}
		}

		if dryRun {
			fmt.Printf("  [DRY]  KHS %s/%s → %s\n", npm, filepath.Base(pdfPath), outputPath)
			stats.SuccessKHS++
			stats.ExtractedKHS = append(stats.ExtractedKHS, fmt.Sprintf("%s/%s", npm, outputFile))
			continue
		}

		// Parse the PDF
		if verbose {
			fmt.Printf("  [EXTRACT] KHS %s/%s ...\n", npm, filepath.Base(pdfPath))
		}

		result, err := extractor.ParseKHS(pdfPath, npm, tahunAjaran, semester)
		if err != nil {
			stats.FailedKHS++
			stats.Errors = append(stats.Errors, fmt.Sprintf("parse khs %s: %v", pdfPath, err))
			fmt.Printf("  [FAIL] KHS %s/%s: %v\n", npm, filepath.Base(pdfPath), err)
			continue
		}

		// Marshal to JSON
		data, err := extractor.MarshalJSON(result)
		if err != nil {
			stats.FailedKHS++
			stats.Errors = append(stats.Errors, fmt.Sprintf("marshal khs %s: %v", pdfPath, err))
			continue
		}

		// Create output directory
		outputDir := filepath.Dir(outputPath)
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			stats.FailedKHS++
			stats.Errors = append(stats.Errors, fmt.Sprintf("mkdir %s: %v", outputDir, err))
			continue
		}

		// Write atomically (tmp + rename)
		tmpPath := outputPath + ".tmp"
		if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
			os.Remove(tmpPath)
			stats.FailedKHS++
			stats.Errors = append(stats.Errors, fmt.Sprintf("write %s: %v", tmpPath, err))
			continue
		}
		if err := os.Rename(tmpPath, outputPath); err != nil {
			os.Remove(tmpPath)
			stats.FailedKHS++
			stats.Errors = append(stats.Errors, fmt.Sprintf("rename %s: %v", outputPath, err))
			continue
		}

		stats.SuccessKHS++
		stats.ExtractedKHS = append(stats.ExtractedKHS, fmt.Sprintf("%s/%s", npm, outputFile))
		fmt.Printf("  [OK]   KHS %s/%s → %s (%d courses)\n", npm, filepath.Base(pdfPath), outputFile, len(result.KHS.MataKuliah))
	}
}

// parseKHSFilename extracts tahun ajaran and semester from a KHS filename.
// Expected format: "2022_2023_GANJIL" → ("2022_2023", "GANJIL").
func parseKHSFilename(base string) (string, string, error) {
	parts := strings.Split(base, "_")
	if len(parts) < 3 {
		return "", "", fmt.Errorf("invalid KHS filename format, expected: tahunAwal_tahunAkhir_SEMESTER")
	}

	semester := parts[len(parts)-1]
	tahunAjaran := strings.Join(parts[:len(parts)-1], "_")
	return tahunAjaran, semester, nil
}

// ternary is a simple inline ternary helper.
func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
