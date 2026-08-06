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
