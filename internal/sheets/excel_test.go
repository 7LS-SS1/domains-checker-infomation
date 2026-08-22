package sheets

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

func workbookBytes(t *testing.T) []byte {
	t.Helper()
	workbook := excelize.NewFile()
	defer workbook.Close()
	workbook.SetSheetName("Sheet1", "Domains")
	if err := workbook.SetSheetRow("Domains", "A1", &[]any{"domain", "renewal_price", "currency"}); err != nil {
		t.Fatal(err)
	}
	if err := workbook.SetSheetRow("Domains", "A2", &[]any{"example.com", "123.450000", "THB"}); err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	if err := workbook.Write(&buffer); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestParseExcelUsesBoundedXLSXReader(t *testing.T) {
	data := workbookBytes(t)
	snapshot, selected, metadata, err := ParseExcel(data, "inventory.xlsx", "Domains", ExcelOptions{MaxBytes: int64(len(data) + 1), MaxUncompressedBytes: 8 << 20, MaxRows: 10, MaxColumns: 10})
	if err != nil {
		t.Fatal(err)
	}
	if selected != "Domains" || metadata.Filename != "inventory.xlsx" || len(snapshot.Values) != 2 || snapshot.Values[1][1] != "123.450000" {
		t.Fatalf("unexpected Excel snapshot: selected=%s metadata=%#v values=%#v", selected, metadata, snapshot.Values)
	}
}

func TestParseExcelRejectsUnsafeInputs(t *testing.T) {
	data := workbookBytes(t)
	if _, _, _, err := ParseExcel(data, "inventory.xlsm", "", ExcelOptions{MaxBytes: 10 << 20}); err == nil {
		t.Fatal("expected macro-enabled extension to be rejected")
	}
	if _, _, _, err := ParseExcel(data, "inventory.xlsx", "", ExcelOptions{MaxBytes: int64(len(data) - 1)}); err == nil {
		t.Fatal("expected oversized workbook to be rejected")
	}
	if _, _, _, err := ParseExcel(data, "inventory.xlsx", "", ExcelOptions{MaxBytes: int64(len(data) + 1), MaxUncompressedBytes: 8 << 20, MaxRows: 1, MaxColumns: 10}); err == nil {
		t.Fatal("expected row limit to be enforced")
	}
}
