package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// workbookFormatSpreadsheetML2003 names the XML workbook that Excel 2003 and many telecom portals export under an .xls or .xml name.
	workbookFormatSpreadsheetML2003 = "spreadsheetml_2003"
	// workbookFormatHint is the upload metadata key that carries the format found by looking inside the file.
	workbookFormatHint = "detected_workbook_format"

	spreadsheetML2003Namespace = "urn:schemas-microsoft-com:office:spreadsheet"
	spreadsheetMLSniffBytes    = 4096
)

// isSpreadsheetML2003File reports whether the stored file is a SpreadsheetML 2003 workbook: an XML document that declares the Excel 2003
// spreadsheet namespace. Only files named .xls or .xml are looked at, and only the first bytes are read. A legacy binary .xls (an OLE
// container), a different XML document, or anything unreadable is not one: it stays registered until an adapter exists for it.
func isSpreadsheetML2003File(path, sourceName string) bool {
	switch strings.ToLower(filepath.Ext(sourceName)) {
	case ".xls", ".xml":
	default:
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()
	buffer := make([]byte, spreadsheetMLSniffBytes)
	read, err := file.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	return looksLikeSpreadsheetML2003(buffer[:read])
}

func looksLikeSpreadsheetML2003(head []byte) bool {
	head = bytes.TrimPrefix(head, []byte{0xEF, 0xBB, 0xBF})
	head = bytes.TrimLeft(head, " \t\r\n")
	if !bytes.HasPrefix(head, []byte("<?xml")) {
		return false
	}
	return bytes.Contains(head, []byte(spreadsheetML2003Namespace))
}
