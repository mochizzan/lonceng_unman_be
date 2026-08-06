package extractor

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	gopdf "github.com/razvandimescu/gopdf/pdf"
)

const (
	maxPDFSize = 50 * 1024 * 1024 // 50MB limit
)

// ReadPDF extracts plain text from a PDF file.
func ReadPDF(path string) (string, error) {
	// Validate file exists and size
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("pdf file not found: %s", path)
		}
		return "", fmt.Errorf("stat pdf: %w", err)
	}
	if info.Size() > maxPDFSize {
		return "", fmt.Errorf("pdf too large: %d bytes (max %d)", info.Size(), maxPDFSize)
	}

	doc, err := gopdf.OpenFile(path)
	if err != nil {
		return "", fmt.Errorf("open pdf (may be corrupted): %w", err)
	}

	text, err := doc.Text()
	if err != nil {
		return "", fmt.Errorf("extract text: %w", err)
	}

	// Normalize: replace raw 0xa0 bytes (non-UTF8 non-breaking space) with regular space
	text = normalizeText(text)

	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("pdf contains no extractable text (may be image-only)")
	}

	return text, nil
}

// PDFWord represents a word with its position in the PDF.
type PDFWord struct {
	Text string
	X    float64
	Y    float64
	Font string
	Size float64
}

// PDFRow represents a row of words sorted by X position.
type PDFRow struct {
	Y     float64
	Words []PDFWord
}

// ReadPDFWithPosition extracts text with positions from a PDF file.
func ReadPDFWithPosition(path string) ([]PDFRow, error) {
	// Validate file exists and size
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("pdf file not found: %s", path)
		}
		return nil, fmt.Errorf("stat pdf: %w", err)
	}
	if info.Size() > maxPDFSize {
		return nil, fmt.Errorf("pdf too large: %d bytes (max %d)", info.Size(), maxPDFSize)
	}

	doc, err := gopdf.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf (may be corrupted): %w", err)
	}

	totalPage := doc.NumPages()
	if totalPage == 0 {
		return nil, fmt.Errorf("pdf has no pages")
	}

	var allRows []PDFRow
	var pageErrors []string

	for pageIndex := 0; pageIndex < totalPage; pageIndex++ {
		p := doc.Page(pageIndex)
		if p == nil {
			pageErrors = append(pageErrors, fmt.Sprintf("page %d is nil", pageIndex+1))
			continue
		}

		spans, err := p.TextSpans()
		if err != nil {
			pageErrors = append(pageErrors, fmt.Sprintf("page %d: %v", pageIndex+1, err))
			continue
		}

		if len(spans) == 0 {
			continue
		}

		// Get page height for Y-axis flip (PDF Y=0 at bottom, screen Y=0 at top)
		mb := p.MediaBox()
		pageHeight := mb[3] // ury

		// Group spans by Y position (rows)
		rowMap := make(map[float64]*PDFRow)
		var yPositions []float64

		for _, span := range spans {
			word := strings.TrimSpace(span.Text)
			if word == "" {
				continue
			}

			// Normalize: replace raw 0xa0 bytes (non-UTF8 non-breaking space) with regular space
			word = normalizeText(word)

			// Flip Y-axis: PDF Y=0 at bottom → screen Y=0 at top
			flippedY := pageHeight - span.Y

			// Round Y to group spans on same line
			yKey := roundY(flippedY)

			if _, exists := rowMap[yKey]; !exists {
				rowMap[yKey] = &PDFRow{Y: yKey}
				yPositions = append(yPositions, yKey)
			}

			rowMap[yKey].Words = append(rowMap[yKey].Words, PDFWord{
				Text: word,
				X:    span.X,
				Y:    flippedY,
				Font: span.Font,
				Size: span.FontSize,
			})
		}

		// Sort rows by Y position (top to bottom)
		sort.Float64s(yPositions)

		for _, y := range yPositions {
			row := rowMap[y]
			// Sort words in row by X position (left to right)
			sort.Slice(row.Words, func(i, j int) bool {
				return row.Words[i].X < row.Words[j].X
			})
			allRows = append(allRows, *row)
		}
	}

	if len(allRows) == 0 {
		return nil, fmt.Errorf("no text extracted from pdf (pages: %d, errors: %v)", totalPage, pageErrors)
	}

	return allRows, nil
}

// roundY rounds Y position to nearest 0.25 to group words on same line.
// Using 0.25 instead of 0.5 for better precision with closely spaced lines.
func roundY(y float64) float64 {
	return float64(int(y*4)) / 4.0
}

// normalizeText replaces invalid UTF-8 bytes (like raw 0xa0) with regular spaces.
// gopdf may return raw bytes that are not valid UTF-8, which Go replaces with U+FFFD.
// This function proactively replaces them before Go's string processing mangles them.
func normalizeText(s string) string {
	var buf []byte
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b == 0xa0 {
			buf = append(buf, ' ')
		} else {
			buf = append(buf, b)
		}
	}
	if buf != nil {
		return string(buf)
	}
	return s
}

// roundX rounds X position to nearest 0.5 to group characters at same position.
func roundX(x float64) float64 {
	return float64(int(x*2)) / 2.0
}

// RowToLine converts a PDFRow to a single line string.
// Characters at the same X position (within tolerance) are joined without spaces,
// since they belong to the same visual token. Different X groups are separated by spaces.
func RowToLine(row PDFRow) string {
	if len(row.Words) == 0 {
		return ""
	}

	// Group words by X position
	type xGroup struct {
		x     float64
		texts []string
	}
	var groups []xGroup
	var lastX float64
	var currentGroup *xGroup

	for _, w := range row.Words {
		xKey := roundX(w.X)
		if currentGroup == nil || abs(xKey-lastX) > 0.5 {
			groups = append(groups, xGroup{x: xKey})
			currentGroup = &groups[len(groups)-1]
			lastX = xKey
		}
		currentGroup.texts = append(currentGroup.texts, w.Text)
	}

	// Join each group without spaces, separate groups with spaces
	var parts []string
	for _, g := range groups {
		parts = append(parts, strings.Join(g.texts, ""))
	}
	return strings.Join(parts, " ")
}

// abs returns the absolute value of a float64.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// RowsToLines converts multiple PDFRows to lines.
func RowsToLines(rows []PDFRow) []string {
	lines := make([]string, len(rows))
	for i, row := range rows {
		lines[i] = RowToLine(row)
	}
	return lines
}

// ExtractTableRows extracts tabular data from PDF rows.
// It identifies table rows by looking for consistent column positions.
func ExtractTableRows(rows []PDFRow, startPattern string) [][]string {
	var tableRows [][]string
	inTable := false

	for _, row := range rows {
		line := RowToLine(row)

		// Detect table start
		if !inTable && strings.Contains(line, startPattern) {
			inTable = true
			continue
		}

		// Detect table end (empty row or next section)
		if inTable && len(row.Words) == 0 {
			break
		}

		if inTable {
			var cells []string
			for _, w := range row.Words {
				cells = append(cells, w.Text)
			}
			tableRows = append(tableRows, cells)
		}
	}

	return tableRows
}

// ColumnBoundary represents a column's X range for position-based extraction.
type ColumnBoundary struct {
	Name  string
	Start float64
	End   float64
}

// FindColumnPositions identifies column X boundaries from a header row.
// It takes the header PDFRow and a list of column header names (in order).
// Strategy:
//  1. Group header characters by X position to find distinct column X values.
//  2. Match column names to these X groups by checking if the joined text
//     starts with the column name (handles "MataKuliah" matching "Mata Kuliah").
//  3. If matching fails, use the X groups directly in order.
//  4. Column boundaries are midpoints between adjacent X positions.
func FindColumnPositions(headerRow PDFRow, columnNames []string) []ColumnBoundary {
	if len(columnNames) == 0 || len(headerRow.Words) == 0 {
		return nil
	}

	// Step 1: Group header words by X position (using roundX tolerance)
	type xGroup struct {
		x     float64
		texts []string
	}
	var groups []xGroup
	var lastX float64
	var currentGroup *xGroup

	for _, w := range headerRow.Words {
		xKey := roundX(w.X)
		if currentGroup == nil || abs(xKey-lastX) > 0.5 {
			groups = append(groups, xGroup{x: xKey})
			currentGroup = &groups[len(groups)-1]
			lastX = xKey
		}
		currentGroup.texts = append(currentGroup.texts, w.Text)
	}

	if len(groups) == 0 {
		return nil
	}

	// Step 2: Match column names to X groups
	// For each column name, find the X group whose joined text starts with it
	// (case-insensitive). This handles "MataKuliah" matching "Mata Kuliah".
	var columnXPositions []float64
	usedGroups := make(map[int]bool)

	for _, name := range columnNames {
		nameLower := strings.ToLower(name)
		nameParts := strings.Fields(nameLower)
		found := false

		for gi, g := range groups {
			if usedGroups[gi] {
				continue
			}
			gText := strings.ToLower(strings.Join(g.texts, ""))

			// Check if the group text starts with the column name
			// Also handle case where column name parts are embedded:
			// "Mata Kuliah" should match "MataKuliah" (joined) or "Mata" + "Kuliah" (split)
			if strings.HasPrefix(gText, nameLower) ||
				strings.HasPrefix(gText, strings.Join(nameParts, "")) {
				columnXPositions = append(columnXPositions, g.x)
				usedGroups[gi] = true
				found = true
				break
			}
		}

		if !found {
			// Fallback: use the Nth X group (column name order matches header order)
			if len(columnXPositions) < len(groups) {
				columnXPositions = append(columnXPositions, groups[len(columnXPositions)].x)
			} else {
				// Not enough groups; use last known X + estimated spacing
				lastX := columnXPositions[len(columnXPositions)-1]
				columnXPositions = append(columnXPositions, lastX+50)
			}
		}
	}

	// Step 3: Build boundaries using midpoints between adjacent X positions
	boundaries := make([]ColumnBoundary, len(columnNames))
	for i, name := range columnNames {
		start := columnXPositions[i]
		var end float64
		if i+1 < len(columnXPositions) {
			end = (columnXPositions[i] + columnXPositions[i+1]) / 2.0
		} else {
			// Last column extends to the right edge
			end = columnXPositions[i] + 500
		}
		boundaries[i] = ColumnBoundary{Name: name, Start: start, End: end}
	}

	return boundaries
}

// ExtractColumnsFromRow extracts text from a PDFRow using column boundaries.
// Returns a map of column name to the reconstructed text of words within that column's X range.
// Characters at the same X position (within tolerance) are joined without spaces,
// then smart word splitting is applied to insert spaces between concatenated words.
func ExtractColumnsFromRow(row PDFRow, boundaries []ColumnBoundary) map[string]string {
	result := make(map[string]string)
	for _, col := range boundaries {
		// Collect words in this column's X range
		var colWords []PDFWord
		for _, w := range row.Words {
			if w.X >= col.Start && w.X < col.End {
				colWords = append(colWords, w)
			}
		}

		if len(colWords) == 0 {
			result[col.Name] = ""
			continue
		}

		// Group words by X position (same logic as RowToLine)
		type xGroup struct {
			x     float64
			texts []string
		}
		var groups []xGroup
		var lastX float64
		var currentGroup *xGroup

		for _, w := range colWords {
			xKey := roundX(w.X)
			if currentGroup == nil || abs(xKey-lastX) > 0.5 {
				groups = append(groups, xGroup{x: xKey})
				currentGroup = &groups[len(groups)-1]
				lastX = xKey
			}
			currentGroup.texts = append(currentGroup.texts, w.Text)
		}

		// Join each group without spaces, then apply smart word splitting
		var parts []string
		for _, g := range groups {
			joined := strings.Join(g.texts, "")
			parts = append(parts, SmartWordSplit(joined))
		}
		result[col.Name] = strings.Join(parts, " ")
	}
	return result
}

// SmartWordSplit inserts spaces between concatenated words in a string.
// It handles common patterns from PDF text extraction:
//   - CamelCase: "TugasAkhir" → "Tugas Akhir"
//   - Punctuation boundaries: "TugasAkhir/Skripsi" → "Tugas Akhir/Skripsi"
//   - Known abbreviations are preserved: "SI40306", "SI-8A"
func SmartWordSplit(s string) string {
	if len(s) <= 1 {
		return s
	}

	// Don't split pure numbers, single chars, or known codes
	if isKnownCode(s) {
		return s
	}

	var result []rune
	runes := []rune(s)

	for i, r := range runes {
		result = append(result, r)

		if i == len(runes)-1 {
			continue
		}

		next := runes[i+1]

		// Insert space before uppercase letter that follows a lowercase letter
		// "TugasAkhir" → "Tugas Akhir"
		if isLowerLetter(r) && IsUpperLetter(next) {
			result = append(result, ' ')
			continue
		}

		// Insert space before uppercase letter that follows a digit
		// "2SI" → "2 SI" (but not "SI4" → "S I 4")
		if isDigit(r) && IsUpperLetter(next) && i+2 < len(runes) && isDigit(runes[i+2]) {
			result = append(result, ' ')
			continue
		}

		// Insert space before digit that follows an uppercase letter (end of abbreviation)
		// "TIM2" → "TIM 2" (but not "SI4" where 4 is part of code)
		if IsUpperLetter(r) && isDigit(next) && i+1 < len(runes) {
			// Check if this is likely an abbreviation followed by a number
			// Look back to see if we have multiple uppercase letters (abbreviation)
			upperCount := 0
			for j := i; j >= 0 && j > i-4; j-- {
				if IsUpperLetter(runes[j]) {
					upperCount++
				} else {
					break
				}
			}
			// If we have 2+ uppercase letters, this is likely end of abbreviation
			if upperCount >= 2 {
				result = append(result, ' ')
			}
		}
	}

	return string(result)
}

// isDigit checks if a rune is a digit.
func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// isKnownCode checks if a string is a known code pattern that should not be split.
// Patterns: SI40306, KK21250723, SI-8A, etc.
func isKnownCode(s string) bool {
	// Pure numbers
	allDigits := true
	for _, r := range s {
		if !isDigit(r) && r != '.' && r != ',' {
			allDigits = false
			break
		}
	}
	if allDigits {
		return true
	}

	// Pattern: 2+ uppercase letters followed by digits (course code)
	// e.g., SI40306, KK21250723, UM21250122
	if len(s) >= 4 {
		upperCount := 0
		for _, r := range s {
			if IsUpperLetter(r) {
				upperCount++
			} else if isDigit(r) {
				break
			} else {
				return false
			}
		}
		if upperCount >= 2 {
			return true
		}
	}

	// Pattern: XX-##A (class code like SI-8A)
	if len(s) >= 3 && strings.Contains(s, "-") {
		parts := strings.Split(s, "-")
		if len(parts) == 2 && len(parts[0]) >= 2 && len(parts[1]) >= 1 {
			return true
		}
	}

	// Single character
	if len([]rune(s)) <= 1 {
		return true
	}

	return false
}

// parseIntSafe safely parses an integer string.
// Handles non-breaking spaces (\xa0) which are common in PDF extractions.
func parseIntSafe(s string) int {
	s = strings.TrimSpace(s)
	// Replace non-breaking spaces (U+00A0) and other Unicode whitespace
	s = strings.NewReplacer(
		"\u00a0", " ",
		"\xa0", " ",
	).Replace(s)
	s = strings.TrimSpace(s)
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

// parseFloatSafe safely parses a float string.
func parseFloatSafe(s string) float64 {
	s = strings.TrimSpace(s)
	// Handle comma as decimal separator
	s = strings.Replace(s, ",", ".", 1)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}
