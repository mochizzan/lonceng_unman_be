package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	gopdf "github.com/razvandimescu/gopdf/pdf"
)

func main() {
	var (
		pdfPath   = flag.String("pdf", "", "Path to PDF file")
		outputDir = flag.String("out", "./raw", "Output directory")
		pageNum   = flag.Int("page", 0, "Only extract specific page (0 = all pages)")
	)
	flag.Parse()

	if *pdfPath == "" {
		fmt.Println("Usage: extractor -pdf <path> [-out <dir>] [-page <n>]")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  extractor -pdf downloads/2211700006/krs/semester_8.pdf")
		fmt.Println("  extractor -pdf downloads/2211700006/krs/semester_8.pdf -page 1")
		fmt.Println("  extractor -pdf downloads/2211700006/krs/semester_8.pdf -out raw")
		os.Exit(1)
	}

	if _, err := os.Stat(*pdfPath); os.IsNotExist(err) {
		fmt.Printf("Error: PDF file not found: %s\n", *pdfPath)
		os.Exit(1)
	}

	doc, err := gopdf.OpenFile(*pdfPath)
	if err != nil {
		fmt.Printf("Error opening PDF: %v\n", err)
		os.Exit(1)
	}

	// Create output directory
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	timestamp := time.Now().Format("20060102_150405")
	baseName := strings.TrimSuffix(filepath.Base(*pdfPath), filepath.Ext(*pdfPath))
	outputFile := filepath.Join(*outputDir, fmt.Sprintf("raw_%s_%s.txt", baseName, timestamp))

	file, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	fmt.Printf("📄 Extracting RAW data from: %s\n", *pdfPath)
	fmt.Printf("📁 Output: %s\n\n", outputFile)

	// Header
	fmt.Fprintln(file, "============================================================")
	fmt.Fprintln(file, "RAW PDF DATA EXTRACTION")
	fmt.Fprintln(file, "============================================================")
	fmt.Fprintf(file, "Filename: %s\n", filepath.Base(*pdfPath))
	fmt.Fprintf(file, "Total Pages: %d\n", doc.NumPages())
	fmt.Fprintf(file, "Extracted At: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintln(file, "============================================================")

	// Section 1: Plain Text (doc.Text())
	fmt.Fprintln(file, "")
	fmt.Fprintln(file, "============================================================")
	fmt.Fprintln(file, "SECTION 1: PLAIN TEXT (doc.Text())")
	fmt.Fprintln(file, "============================================================")
	fmt.Fprintln(file, "")

	plainText, err := doc.Text()
	if err != nil {
		fmt.Fprintf(file, "Error: %v\n", err)
	} else {
		fmt.Fprintln(file, plainText)
	}

	// Section 2: Raw Spans per Page
	startPage := 0
	endPage := doc.NumPages()
	if *pageNum > 0 && *pageNum <= doc.NumPages() {
		startPage = *pageNum - 1
		endPage = *pageNum
	}

	for pageIndex := startPage; pageIndex < endPage; pageIndex++ {
		p := doc.Page(pageIndex)
		if p == nil {
			continue
		}

		mb := p.MediaBox()
		pageWidth := mb[2] - mb[0]
		pageHeight := mb[3] - mb[1]

		fmt.Fprintln(file, "")
		fmt.Fprintln(file, "============================================================")
		fmt.Fprintf(file, "SECTION 2: PAGE %d - RAW SPANS (p.TextSpans())\n", pageIndex+1)
		fmt.Fprintln(file, "============================================================")
		fmt.Fprintf(file, "Page Width:  %.2f\n", pageWidth)
		fmt.Fprintf(file, "Page Height: %.2f\n", pageHeight)
		fmt.Fprintln(file, "")
		fmt.Fprintln(file, "Note: Y=0 is at BOTTOM of page (PDF coordinate system)")
		fmt.Fprintln(file, "      To flip Y: flippedY = pageHeight - Y")
		fmt.Fprintln(file, "")
		fmt.Fprintln(file, "------------------------------------------------------------")
		fmt.Fprintln(file, "RAW SPANS (as returned by gopdf)")
		fmt.Fprintln(file, "------------------------------------------------------------")
		fmt.Fprintf(file, "%-6s %-50s %10s %10s %15s %8s\n",
			"INDEX", "TEXT", "X", "Y", "FONT", "F_SIZE")
		fmt.Fprintln(file, strings.Repeat("-", 110))

		spans, err := p.TextSpans()
		if err != nil {
			fmt.Fprintf(file, "Error: %v\n", err)
			continue
		}

		for i, span := range spans {
			fmt.Fprintf(file, "%-6d %-50s %10.2f %10.2f %15s %8.2f\n",
				i, truncate(span.Text, 48), span.X, span.Y, span.Font, span.FontSize)
		}

		// Section 3: Grouped by Rows (using roundY)
		fmt.Fprintln(file, "")
		fmt.Fprintln(file, "------------------------------------------------------------")
		fmt.Fprintln(file, "GROUPED BY ROWS (Y rounded to 0.25, sorted top-to-bottom)")
		fmt.Fprintln(file, "------------------------------------------------------------")

		rowMap := make(map[float64][]gopdf.TextSpan)
		var yKeys []float64

		for _, span := range spans {
			flippedY := pageHeight - span.Y
			yKey := float64(int(flippedY*4)) / 4.0
			if _, exists := rowMap[yKey]; !exists {
				yKeys = append(yKeys, yKey)
			}
			rowMap[yKey] = append(rowMap[yKey], span)
		}

		sort.Float64s(yKeys)

		for _, yKey := range yKeys {
			rowSpans := rowMap[yKey]
			sort.Slice(rowSpans, func(i, j int) bool {
				return rowSpans[i].X < rowSpans[j].X
			})

			fmt.Fprintf(file, "\n[Row Y=%07.2f] ", yKey)
			for _, span := range rowSpans {
				fmt.Fprintf(file, "[X=%07.2f]\"%s\" ", span.X, span.Text)
			}
			fmt.Fprintln(file, "")
		}

		// Section 4: Lines (after RowToLine logic)
		fmt.Fprintln(file, "")
		fmt.Fprintln(file, "------------------------------------------------------------")
		fmt.Fprintln(file, "RECONSTRUCTED LINES (RowToLine logic applied)")
		fmt.Fprintln(file, "------------------------------------------------------------")

		for _, yKey := range yKeys {
			rowSpans := rowMap[yKey]
			sort.Slice(rowSpans, func(i, j int) bool {
				return rowSpans[i].X < rowSpans[j].X
			})

			// Group by X (roundX)
			var groups [][]gopdf.TextSpan
			var currentGroup []gopdf.TextSpan
			var lastX float64 = -9999

			for _, span := range rowSpans {
				xKey := float64(int(span.X*2)) / 2.0
				if len(currentGroup) > 0 && abs(xKey-lastX) > 0.5 {
					groups = append(groups, currentGroup)
					currentGroup = nil
				}
				currentGroup = append(currentGroup, span)
				lastX = xKey
			}
			if len(currentGroup) > 0 {
				groups = append(groups, currentGroup)
			}

			// Join groups
			var line string
			for gi, group := range groups {
				var parts []string
				for _, span := range group {
					parts = append(parts, span.Text)
				}
				joined := strings.Join(parts, "")
				if gi > 0 {
					line += " "
				}
				line += joined
			}

			fmt.Fprintf(file, "[Y=%07.2f] %s\n", yKey, line)
		}
	}

	fmt.Fprintf(file, "\n============================================================\n")
	fmt.Fprintln(file, "END OF RAW DATA")
	fmt.Fprintln(file, "============================================================")

	fmt.Printf("✅ RAW data written to: %s\n", outputFile)
	fmt.Printf("📊 File size: %d bytes\n", fileSize(outputFile))
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) > n {
		return string(runes[:n-3]) + "..."
	}
	return s
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
