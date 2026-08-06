package extractor

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/ledongthuc/pdf"
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

	f, r, err := pdf.Open(path)
	if err != nil {
		return "", fmt.Errorf("open pdf (may be corrupted): %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("get plain text: %w", err)
	}
	_, err = buf.ReadFrom(b)
	if err != nil {
		return "", fmt.Errorf("read buffer: %w", err)
	}

	text := buf.String()
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

	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf (may be corrupted): %w", err)
	}
	defer f.Close()

	totalPage := r.NumPage()
	if totalPage == 0 {
		return nil, fmt.Errorf("pdf has no pages")
	}

	var allRows []PDFRow
	var pageErrors []string

	for pageIndex := 1; pageIndex <= totalPage; pageIndex++ {
		p := r.Page(pageIndex)
		if p.V.IsNull() {
			pageErrors = append(pageErrors, fmt.Sprintf("page %d is null", pageIndex))
			continue
		}

		content := p.Content()
		texts := content.Text

		// Group words by Y position (rows)
		rowMap := make(map[float64]*PDFRow)
		var yPositions []float64

		for _, t := range texts {
			word := strings.TrimSpace(t.S)
			if word == "" {
				continue
			}

			// Round Y to group words on same line
			yKey := roundY(t.Y)

			if _, exists := rowMap[yKey]; !exists {
				rowMap[yKey] = &PDFRow{Y: yKey}
				yPositions = append(yPositions, yKey)
			}

			rowMap[yKey].Words = append(rowMap[yKey].Words, PDFWord{
				Text: word,
				X:    t.X,
				Y:    t.Y,
				Font: t.Font,
				Size: t.FontSize,
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
// matching how RowToLine reconstructs words from individual characters.
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

		// Join each group without spaces, separate groups with spaces
		var parts []string
		for _, g := range groups {
			parts = append(parts, strings.Join(g.texts, ""))
		}
		result[col.Name] = strings.Join(parts, " ")
	}
	return result
}

// parseIntSafe safely parses an integer string.
func parseIntSafe(s string) int {
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
