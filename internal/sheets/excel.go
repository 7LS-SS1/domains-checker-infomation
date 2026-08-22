package sheets

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

type ExcelOptions struct {
	MaxBytes             int64
	MaxUncompressedBytes int64
	MaxRows              int
	MaxColumns           int
}

type ExcelMetadata struct {
	Filename   string   `json:"filename"`
	SizeBytes  int      `json:"size_bytes"`
	SheetNames []string `json:"sheet_names"`
}

func ParseExcel(data []byte, filename, requestedSheet string, options ExcelOptions) (Snapshot, string, ExcelMetadata, error) {
	filename = strings.TrimSpace(filepath.Base(filename))
	if strings.ToLower(filepath.Ext(filename)) != ".xlsx" {
		return Snapshot{}, "", ExcelMetadata{}, fmt.Errorf("%w: only .xlsx files are supported", ErrValidation)
	}
	if options.MaxBytes <= 0 {
		options.MaxBytes = 10 << 20
	}
	if int64(len(data)) == 0 || int64(len(data)) > options.MaxBytes || len(data) < 4 || !bytes.Equal(data[:2], []byte("PK")) {
		return Snapshot{}, "", ExcelMetadata{}, fmt.Errorf("%w: invalid or oversized .xlsx file", ErrValidation)
	}
	if options.MaxUncompressedBytes <= 0 {
		options.MaxUncompressedBytes = 64 << 20
	}
	if options.MaxRows <= 0 {
		options.MaxRows = 20000
	}
	if options.MaxColumns <= 0 {
		options.MaxColumns = 100
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{
		UnzipSizeLimit:    options.MaxUncompressedBytes,
		UnzipXMLSizeLimit: options.MaxUncompressedBytes,
	})
	if err != nil {
		return Snapshot{}, "", ExcelMetadata{}, fmt.Errorf("%w: unreadable .xlsx workbook", ErrValidation)
	}
	defer func() { _ = workbook.Close() }()
	sheets := workbook.GetSheetList()
	if len(sheets) == 0 {
		return Snapshot{}, "", ExcelMetadata{}, fmt.Errorf("%w: workbook has no worksheets", ErrValidation)
	}
	selected := strings.TrimSpace(requestedSheet)
	if selected == "" {
		selected = sheets[0]
	} else {
		found := false
		for _, name := range sheets {
			if name == selected {
				found = true
				break
			}
		}
		if !found {
			return Snapshot{}, "", ExcelMetadata{}, fmt.Errorf("%w: worksheet not found", ErrValidation)
		}
	}
	iterator, err := workbook.Rows(selected)
	if err != nil {
		return Snapshot{}, "", ExcelMetadata{}, fmt.Errorf("%w: worksheet cannot be read", ErrValidation)
	}
	defer func() { _ = iterator.Close() }()
	values := make([][]string, 0, min(options.MaxRows, 256))
	for iterator.Next() {
		if len(values) >= options.MaxRows {
			return Snapshot{}, "", ExcelMetadata{}, fmt.Errorf("%w: worksheet exceeds row limit", ErrValidation)
		}
		columns, columnErr := iterator.Columns()
		if columnErr != nil {
			return Snapshot{}, "", ExcelMetadata{}, fmt.Errorf("%w: worksheet row cannot be read", ErrValidation)
		}
		if len(columns) > options.MaxColumns {
			return Snapshot{}, "", ExcelMetadata{}, fmt.Errorf("%w: worksheet exceeds column limit", ErrValidation)
		}
		for index := range columns {
			columns[index] = strings.TrimSpace(columns[index])
		}
		values = append(values, columns)
	}
	if err := iterator.Error(); err != nil {
		return Snapshot{}, "", ExcelMetadata{}, fmt.Errorf("%w: worksheet iteration failed", ErrValidation)
	}
	metadata := ExcelMetadata{Filename: filename, SizeBytes: len(data), SheetNames: sheets}
	return Snapshot{Values: values}, selected, metadata, nil
}
