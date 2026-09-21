package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	maxSQLiteInventoryBytes     = 64 << 20
	maxSQLitePagesVisited       = 4096
	maxSQLiteSchemaObjects      = 2048
	maxSQLiteSchemaRecordBytes  = 1 << 20
	maxSQLiteSchemaPayloadBytes = 8 << 20
	maxSQLiteNameBytes          = 512
	maxSQLiteBTreeDepth         = 64
	maxSQLiteCountedTables      = 256
)

var errSQLiteInventoryLimit = errors.New("SQLite inventory resource limit reached")

type sqliteInventoryReader struct {
	file          *os.File
	pageSize      int
	usableSize    int
	pageCount     uint32
	bytesRead     int64
	pagesRead     int
	pageCache     map[uint32][]byte
	textEncoding  uint32
	schemaPayload int64
}

type sqliteBTreePage struct {
	pageType       byte
	headerOffset   int
	headerBytes    int
	cellCount      int
	cellPointerEnd int
	rightMostPage  uint32
}

type sqliteSchemaObject struct {
	objectType    string
	name          string
	tableName     string
	rootPage      uint32
	sqlBytes      int
	virtualTable  bool
	withoutRowID  bool
	internal      bool
	schemaSQLSeen bool
}

type sqliteRecordValue struct {
	kind    string
	integer int64
	text    string
}

func extractSQLiteMetadata(path string) (map[string]any, []string, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("stat database: %w", err)
	}
	if info.Size() < 100 {
		return nil, nil, fmt.Errorf("file is shorter than the 100-byte SQLite database header")
	}

	var header [100]byte
	if _, err := file.ReadAt(header[:], 0); err != nil {
		return nil, nil, fmt.Errorf("read database header: %w", err)
	}
	if !bytes.Equal(header[:16], []byte("SQLite format 3\x00")) {
		return nil, nil, fmt.Errorf("missing SQLite format 3 header")
	}

	pageSize := int(binary.BigEndian.Uint16(header[16:18]))
	if pageSize == 1 {
		pageSize = 65536
	}
	if pageSize < 512 || pageSize > 65536 || pageSize&(pageSize-1) != 0 {
		return nil, nil, fmt.Errorf("invalid database page size %d", pageSize)
	}
	reservedBytes := int(header[20])
	usableSize := pageSize - reservedBytes
	if usableSize < 480 {
		return nil, nil, fmt.Errorf("usable page size %d is below SQLite's 480-byte minimum", usableSize)
	}
	if info.Size() < int64(pageSize) || info.Size()%int64(pageSize) != 0 {
		return nil, nil, fmt.Errorf("file size %d is not a positive multiple of page size %d", info.Size(), pageSize)
	}
	actualPageCount64 := info.Size() / int64(pageSize)
	if actualPageCount64 > math.MaxUint32-1 {
		return nil, nil, fmt.Errorf("database has %d pages, above the SQLite page-number limit", actualPageCount64)
	}
	actualPageCount := uint32(actualPageCount64)
	writeVersion := header[18]
	readVersion := header[19]
	if (writeVersion != 1 && writeVersion != 2) || (readVersion != 1 && readVersion != 2) {
		return nil, nil, fmt.Errorf("unsupported file format versions write=%d read=%d", writeVersion, readVersion)
	}
	if header[21] != 64 || header[22] != 32 || header[23] != 32 {
		return nil, nil, fmt.Errorf("invalid embedded payload fractions %d/%d/%d", header[21], header[22], header[23])
	}

	declaredPageCount := binary.BigEndian.Uint32(header[28:32])
	freelistTrunk := binary.BigEndian.Uint32(header[32:36])
	freelistPages := binary.BigEndian.Uint32(header[36:40])
	schemaFormat := binary.BigEndian.Uint32(header[44:48])
	if schemaFormat > 4 {
		return nil, nil, fmt.Errorf("unsupported schema format %d", schemaFormat)
	}
	textEncoding := binary.BigEndian.Uint32(header[56:60])
	if textEncoding > 3 {
		return nil, nil, fmt.Errorf("unsupported database text encoding %d", textEncoding)
	}
	if freelistTrunk > actualPageCount || freelistPages > actualPageCount {
		return nil, nil, fmt.Errorf("freelist declaration exceeds the available database pages")
	}

	metadata := mediaMetadataBase("sqlite3")
	metadata["inspection_mode"] = "raw_file_read_only"
	metadata["sql_executed"] = false
	metadata["extensions_loaded"] = false
	metadata["sidecar_files_opened"] = false
	metadata["data_values_read"] = false
	metadata["schema_sql_retained"] = false
	metadata["page_size_bytes"] = pageSize
	metadata["usable_page_size_bytes"] = usableSize
	metadata["reserved_bytes_per_page"] = reservedBytes
	metadata["file_size_bytes"] = info.Size()
	metadata["actual_page_count"] = actualPageCount
	metadata["declared_page_count"] = declaredPageCount
	metadata["write_version"] = int(writeVersion)
	metadata["read_version"] = int(readVersion)
	metadata["wal_mode_header"] = writeVersion == 2 || readVersion == 2
	metadata["file_change_counter"] = binary.BigEndian.Uint32(header[24:28])
	metadata["freelist_trunk_page"] = freelistTrunk
	metadata["freelist_page_count"] = freelistPages
	metadata["schema_cookie"] = binary.BigEndian.Uint32(header[40:44])
	metadata["schema_format"] = schemaFormat
	metadata["default_page_cache_size"] = int32(binary.BigEndian.Uint32(header[48:52]))
	metadata["largest_root_btree_page"] = binary.BigEndian.Uint32(header[52:56])
	metadata["text_encoding"] = sqliteEncodingName(textEncoding)
	metadata["user_version"] = binary.BigEndian.Uint32(header[60:64])
	metadata["incremental_vacuum"] = binary.BigEndian.Uint32(header[64:68]) != 0
	metadata["application_id"] = binary.BigEndian.Uint32(header[68:72])
	metadata["version_valid_for"] = binary.BigEndian.Uint32(header[92:96])
	writeLibraryVersion := binary.BigEndian.Uint32(header[96:100])
	metadata["write_library_version_number"] = writeLibraryVersion
	metadata["write_library_version"] = sqliteVersionString(writeLibraryVersion)

	warnings := []string{}
	inventoryComplete := true
	if declaredPageCount != 0 && declaredPageCount != actualPageCount {
		warnings = append(warnings, fmt.Sprintf("SQLite header declares %d pages but the uploaded file contains %d", declaredPageCount, actualPageCount))
		inventoryComplete = false
	}
	if writeVersion != readVersion {
		warnings = append(warnings, "SQLite read and write format versions differ")
		inventoryComplete = false
	}
	if writeVersion == 2 || readVersion == 2 {
		warnings = append(warnings, "SQLite WAL mode is declared; uncheckpointed frames from a separate -wal file are not present in this single-file inventory")
	}
	if !allZero(header[72:92]) {
		warnings = append(warnings, "SQLite reserved header expansion bytes are non-zero")
		inventoryComplete = false
	}

	reader := &sqliteInventoryReader{
		file:         file,
		pageSize:     pageSize,
		usableSize:   usableSize,
		pageCount:    actualPageCount,
		pageCache:    map[uint32][]byte{},
		textEncoding: textEncoding,
	}
	objects, schemaComplete, schemaWarnings, err := reader.readSchema()
	if err != nil {
		return nil, nil, err
	}
	warnings = appendUniqueWarnings(warnings, schemaWarnings...)
	if !schemaComplete {
		inventoryComplete = false
	}

	objectSummaries := make([]map[string]any, 0, len(objects))
	objectTypeCounts := map[string]int{"table": 0, "index": 0, "view": 0, "trigger": 0}
	virtualTableCount := 0
	withoutRowIDCount := 0
	internalObjectCount := 0
	seenRootPages := map[uint32]string{}
	for _, object := range objects {
		if object.rootPage > actualPageCount {
			warnings = appendUniqueWarnings(warnings, fmt.Sprintf("SQLite schema object %q references unavailable root page %d", object.name, object.rootPage))
			inventoryComplete = false
		}
		if object.rootPage != 0 {
			if priorName, exists := seenRootPages[object.rootPage]; exists {
				warnings = appendUniqueWarnings(warnings, fmt.Sprintf("SQLite schema objects %q and %q reuse root page %d", priorName, object.name, object.rootPage))
				inventoryComplete = false
			} else {
				seenRootPages[object.rootPage] = object.name
			}
		}
		objectTypeCounts[object.objectType]++
		if object.virtualTable {
			virtualTableCount++
		}
		if object.withoutRowID {
			withoutRowIDCount++
		}
		if object.internal {
			internalObjectCount++
		}
		objectSummaries = append(objectSummaries, map[string]any{
			"type":            object.objectType,
			"name":            object.name,
			"table_name":      object.tableName,
			"root_page":       object.rootPage,
			"sql_bytes":       object.sqlBytes,
			"virtual_table":   object.virtualTable,
			"without_rowid":   object.withoutRowID,
			"internal_object": object.internal,
		})
	}
	metadata["schema_object_count"] = len(objects)
	metadata["schema_objects"] = objectSummaries
	metadata["schema_object_names_retained"] = true
	metadata["table_count"] = objectTypeCounts["table"]
	metadata["index_count"] = objectTypeCounts["index"]
	metadata["view_count"] = objectTypeCounts["view"]
	metadata["trigger_count"] = objectTypeCounts["trigger"]
	metadata["virtual_table_count"] = virtualTableCount
	metadata["without_rowid_table_count"] = withoutRowIDCount
	metadata["internal_schema_object_count"] = internalObjectCount
	if objectTypeCounts["trigger"] > 0 || virtualTableCount > 0 {
		warnings = appendUniqueWarnings(warnings, "SQLite schema contains trigger or virtual-table definitions; definitions were inventoried but never executed")
	}

	tableSummaries, rowCountsComplete, rowWarnings := reader.countTables(objects)
	warnings = appendUniqueWarnings(warnings, rowWarnings...)
	metadata["table_summaries"] = tableSummaries
	metadata["table_row_counts_complete"] = rowCountsComplete
	metadata["column_inventory_complete"] = false
	metadata["foreign_key_inventory_complete"] = false
	metadata["selected_rows_extracted"] = false
	metadata["pages_visited"] = reader.pagesRead
	metadata["bytes_inspected"] = reader.bytesRead
	metadata["inventory_complete"] = inventoryComplete && rowCountsComplete
	metadata["content_validation"] = "header_schema_btree_and_bounded_row_counts"
	return metadata, warnings, nil
}

func (reader *sqliteInventoryReader) readSchema() ([]sqliteSchemaObject, bool, []string, error) {
	type pageVisit struct {
		page  uint32
		depth int
	}
	queue := []pageVisit{{page: 1, depth: 1}}
	visited := map[uint32]struct{}{}
	objects := []sqliteSchemaObject{}
	complete := true
	warnings := []string{}

	for len(queue) > 0 {
		visit := queue[0]
		queue = queue[1:]
		if visit.depth > maxSQLiteBTreeDepth {
			return nil, false, nil, fmt.Errorf("sqlite_schema b-tree exceeds depth limit %d", maxSQLiteBTreeDepth)
		}
		if _, exists := visited[visit.page]; exists {
			return nil, false, nil, fmt.Errorf("sqlite_schema b-tree contains a page cycle at page %d", visit.page)
		}
		visited[visit.page] = struct{}{}
		page, err := reader.readPage(visit.page)
		if err != nil {
			return nil, false, nil, err
		}
		btree, err := reader.parseBTreePage(page, visit.page)
		if err != nil {
			return nil, false, nil, fmt.Errorf("parse sqlite_schema page %d: %w", visit.page, err)
		}
		switch btree.pageType {
		case 0x05:
			children, err := reader.tableInteriorChildren(page, btree)
			if err != nil {
				return nil, false, nil, fmt.Errorf("parse sqlite_schema children on page %d: %w", visit.page, err)
			}
			for _, child := range children {
				queue = append(queue, pageVisit{page: child, depth: visit.depth + 1})
			}
		case 0x0d:
			for cellIndex := 0; cellIndex < btree.cellCount; cellIndex++ {
				if len(objects) >= maxSQLiteSchemaObjects {
					return nil, false, nil, fmt.Errorf("sqlite_schema exceeds object limit %d", maxSQLiteSchemaObjects)
				}
				payload, err := reader.tableLeafPayload(page, btree, cellIndex)
				if err != nil {
					return nil, false, nil, fmt.Errorf("read sqlite_schema cell %d on page %d: %w", cellIndex, visit.page, err)
				}
				reader.schemaPayload += int64(len(payload))
				if reader.schemaPayload > maxSQLiteSchemaPayloadBytes {
					return nil, false, nil, fmt.Errorf("sqlite_schema payload exceeds inventory limit %d", maxSQLiteSchemaPayloadBytes)
				}
				object, err := decodeSQLiteSchemaObject(payload, reader.textEncoding)
				if err != nil {
					return nil, false, nil, fmt.Errorf("decode sqlite_schema cell %d on page %d: %w", cellIndex, visit.page, err)
				}
				objects = append(objects, object)
			}
		default:
			return nil, false, nil, fmt.Errorf("sqlite_schema page %d has non-table b-tree type 0x%02x", visit.page, btree.pageType)
		}
	}
	return objects, complete, warnings, nil
}

func (reader *sqliteInventoryReader) countTables(objects []sqliteSchemaObject) ([]map[string]any, bool, []string) {
	summaries := []map[string]any{}
	complete := true
	warnings := []string{}
	counted := 0
	for _, object := range objects {
		if object.objectType != "table" || object.rootPage == 0 || object.virtualTable {
			continue
		}
		if counted >= maxSQLiteCountedTables {
			complete = false
			warnings = appendUniqueWarnings(warnings, fmt.Sprintf("SQLite table row-count inventory is capped at %d tables", maxSQLiteCountedTables))
			break
		}
		counted++
		rowCount, pages, countComplete, err := reader.countTableBTree(object.rootPage, object.withoutRowID)
		summary := map[string]any{
			"name":           object.name,
			"root_page":      object.rootPage,
			"without_rowid":  object.withoutRowID,
			"pages_visited":  pages,
			"count_complete": countComplete && err == nil,
		}
		if err == nil {
			summary["row_count"] = rowCount
		} else {
			complete = false
			summary["count_error"] = err.Error()
			warnings = appendUniqueWarnings(warnings, fmt.Sprintf("SQLite row count for table %q is incomplete: %v", object.name, err))
		}
		summaries = append(summaries, summary)
	}
	return summaries, complete, warnings
}

func (reader *sqliteInventoryReader) countTableBTree(root uint32, withoutRowID bool) (uint64, int, bool, error) {
	type pageVisit struct {
		page  uint32
		depth int
	}
	queue := []pageVisit{{page: root, depth: 1}}
	visited := map[uint32]struct{}{}
	var rows uint64
	for len(queue) > 0 {
		visit := queue[0]
		queue = queue[1:]
		if visit.depth > maxSQLiteBTreeDepth {
			return rows, len(visited), false, fmt.Errorf("b-tree exceeds depth limit %d", maxSQLiteBTreeDepth)
		}
		if _, exists := visited[visit.page]; exists {
			return rows, len(visited), false, fmt.Errorf("b-tree page cycle at page %d", visit.page)
		}
		visited[visit.page] = struct{}{}
		page, err := reader.readPage(visit.page)
		if err != nil {
			return rows, len(visited), false, err
		}
		btree, err := reader.parseBTreePage(page, visit.page)
		if err != nil {
			return rows, len(visited), false, err
		}
		if withoutRowID {
			switch btree.pageType {
			case 0x02:
				rows += uint64(btree.cellCount)
				children, err := reader.indexInteriorChildren(page, btree)
				if err != nil {
					return rows, len(visited), false, err
				}
				for _, child := range children {
					queue = append(queue, pageVisit{page: child, depth: visit.depth + 1})
				}
			case 0x0a:
				rows += uint64(btree.cellCount)
			default:
				return rows, len(visited), false, fmt.Errorf("WITHOUT ROWID root uses unexpected b-tree type 0x%02x", btree.pageType)
			}
			continue
		}
		switch btree.pageType {
		case 0x05:
			children, err := reader.tableInteriorChildren(page, btree)
			if err != nil {
				return rows, len(visited), false, err
			}
			for _, child := range children {
				queue = append(queue, pageVisit{page: child, depth: visit.depth + 1})
			}
		case 0x0d:
			rows += uint64(btree.cellCount)
		default:
			return rows, len(visited), false, fmt.Errorf("rowid table uses unexpected b-tree type 0x%02x", btree.pageType)
		}
	}
	return rows, len(visited), true, nil
}

func (reader *sqliteInventoryReader) readPage(pageNumber uint32) ([]byte, error) {
	if pageNumber == 0 || pageNumber > reader.pageCount {
		return nil, fmt.Errorf("page %d is outside available range 1-%d", pageNumber, reader.pageCount)
	}
	if cached, ok := reader.pageCache[pageNumber]; ok {
		return cached, nil
	}
	if reader.pagesRead >= maxSQLitePagesVisited || reader.bytesRead+int64(reader.pageSize) > maxSQLiteInventoryBytes {
		return nil, fmt.Errorf("%w: pages=%d bytes=%d", errSQLiteInventoryLimit, reader.pagesRead, reader.bytesRead)
	}
	page := make([]byte, reader.pageSize)
	offset := int64(pageNumber-1) * int64(reader.pageSize)
	if _, err := reader.file.ReadAt(page, offset); err != nil {
		return nil, fmt.Errorf("read page %d: %w", pageNumber, err)
	}
	reader.pagesRead++
	reader.bytesRead += int64(reader.pageSize)
	reader.pageCache[pageNumber] = page
	return page, nil
}

func (reader *sqliteInventoryReader) parseBTreePage(page []byte, pageNumber uint32) (sqliteBTreePage, error) {
	headerOffset := 0
	if pageNumber == 1 {
		headerOffset = 100
	}
	if headerOffset+8 > reader.usableSize {
		return sqliteBTreePage{}, errors.New("b-tree header exceeds usable page bytes")
	}
	pageType := page[headerOffset]
	headerBytes := 8
	if pageType == 0x02 || pageType == 0x05 {
		headerBytes = 12
	} else if pageType != 0x0a && pageType != 0x0d {
		return sqliteBTreePage{}, fmt.Errorf("invalid b-tree page type 0x%02x", pageType)
	}
	cellCount := int(binary.BigEndian.Uint16(page[headerOffset+3 : headerOffset+5]))
	cellPointerEnd := headerOffset + headerBytes + cellCount*2
	if cellPointerEnd > reader.usableSize {
		return sqliteBTreePage{}, fmt.Errorf("cell pointer array for %d cells exceeds usable page bytes", cellCount)
	}
	cellContentStart := int(binary.BigEndian.Uint16(page[headerOffset+5 : headerOffset+7]))
	if cellContentStart == 0 && reader.pageSize == 65536 {
		cellContentStart = 65536
	}
	if cellContentStart < cellPointerEnd || cellContentStart > reader.usableSize {
		return sqliteBTreePage{}, fmt.Errorf("cell content offset %d is outside valid range %d-%d", cellContentStart, cellPointerEnd, reader.usableSize)
	}
	if page[headerOffset+7] > 60 {
		return sqliteBTreePage{}, fmt.Errorf("fragmented free-byte count %d exceeds 60", page[headerOffset+7])
	}
	result := sqliteBTreePage{
		pageType:       pageType,
		headerOffset:   headerOffset,
		headerBytes:    headerBytes,
		cellCount:      cellCount,
		cellPointerEnd: cellPointerEnd,
	}
	if headerBytes == 12 {
		result.rightMostPage = binary.BigEndian.Uint32(page[headerOffset+8 : headerOffset+12])
		if result.rightMostPage == 0 || result.rightMostPage > reader.pageCount {
			return sqliteBTreePage{}, fmt.Errorf("right-most child page %d is out of range", result.rightMostPage)
		}
	}
	return result, nil
}

func (reader *sqliteInventoryReader) cellOffset(page []byte, btree sqliteBTreePage, index int) (int, error) {
	if index < 0 || index >= btree.cellCount {
		return 0, fmt.Errorf("cell index %d is out of range", index)
	}
	pointerOffset := btree.headerOffset + btree.headerBytes + index*2
	cellOffset := int(binary.BigEndian.Uint16(page[pointerOffset : pointerOffset+2]))
	if cellOffset < btree.cellPointerEnd || cellOffset >= reader.usableSize {
		return 0, fmt.Errorf("cell %d offset %d is outside usable content", index, cellOffset)
	}
	return cellOffset, nil
}

func (reader *sqliteInventoryReader) tableInteriorChildren(page []byte, btree sqliteBTreePage) ([]uint32, error) {
	if btree.pageType != 0x05 {
		return nil, fmt.Errorf("page type 0x%02x is not an interior table b-tree", btree.pageType)
	}
	children := make([]uint32, 0, btree.cellCount+1)
	for index := 0; index < btree.cellCount; index++ {
		offset, err := reader.cellOffset(page, btree, index)
		if err != nil {
			return nil, err
		}
		if offset+4 > reader.usableSize {
			return nil, fmt.Errorf("interior table cell %d is truncated", index)
		}
		child := binary.BigEndian.Uint32(page[offset : offset+4])
		if child == 0 || child > reader.pageCount {
			return nil, fmt.Errorf("interior table child page %d is out of range", child)
		}
		children = append(children, child)
	}
	return append(children, btree.rightMostPage), nil
}

func (reader *sqliteInventoryReader) indexInteriorChildren(page []byte, btree sqliteBTreePage) ([]uint32, error) {
	if btree.pageType != 0x02 {
		return nil, fmt.Errorf("page type 0x%02x is not an interior index b-tree", btree.pageType)
	}
	children := make([]uint32, 0, btree.cellCount+1)
	for index := 0; index < btree.cellCount; index++ {
		offset, err := reader.cellOffset(page, btree, index)
		if err != nil {
			return nil, err
		}
		if offset+4 > reader.usableSize {
			return nil, fmt.Errorf("interior index cell %d is truncated", index)
		}
		child := binary.BigEndian.Uint32(page[offset : offset+4])
		if child == 0 || child > reader.pageCount {
			return nil, fmt.Errorf("interior index child page %d is out of range", child)
		}
		children = append(children, child)
	}
	return append(children, btree.rightMostPage), nil
}

func (reader *sqliteInventoryReader) tableLeafPayload(page []byte, btree sqliteBTreePage, cellIndex int) ([]byte, error) {
	if btree.pageType != 0x0d {
		return nil, fmt.Errorf("page type 0x%02x is not a leaf table b-tree", btree.pageType)
	}
	offset, err := reader.cellOffset(page, btree, cellIndex)
	if err != nil {
		return nil, err
	}
	payloadSize, consumed, err := readSQLiteVarint(page[offset:reader.usableSize])
	if err != nil {
		return nil, fmt.Errorf("read payload size: %w", err)
	}
	offset += consumed
	_, consumed, err = readSQLiteVarint(page[offset:reader.usableSize])
	if err != nil {
		return nil, fmt.Errorf("read rowid: %w", err)
	}
	offset += consumed
	if payloadSize > maxSQLiteSchemaRecordBytes {
		return nil, fmt.Errorf("schema record is %d bytes; limit is %d", payloadSize, maxSQLiteSchemaRecordBytes)
	}
	localBytes := sqliteTableLeafLocalPayload(payloadSize, uint64(reader.usableSize))
	if localBytes > uint64(reader.usableSize-offset) {
		return nil, errors.New("local schema payload exceeds the containing page")
	}
	payload := make([]byte, 0, int(payloadSize))
	payload = append(payload, page[offset:offset+int(localBytes)]...)
	if localBytes == payloadSize {
		return payload, nil
	}
	offset += int(localBytes)
	if offset+4 > reader.usableSize {
		return nil, errors.New("schema overflow pointer is truncated")
	}
	nextPage := binary.BigEndian.Uint32(page[offset : offset+4])
	remaining := payloadSize - localBytes
	visited := map[uint32]struct{}{}
	for remaining > 0 {
		if nextPage == 0 {
			return nil, errors.New("schema overflow chain ended before the declared payload")
		}
		if _, exists := visited[nextPage]; exists {
			return nil, fmt.Errorf("schema overflow page cycle at page %d", nextPage)
		}
		visited[nextPage] = struct{}{}
		overflowPage, err := reader.readPage(nextPage)
		if err != nil {
			return nil, err
		}
		nextPage = binary.BigEndian.Uint32(overflowPage[:4])
		chunk := uint64(reader.usableSize - 4)
		if chunk > remaining {
			chunk = remaining
		}
		payload = append(payload, overflowPage[4:4+int(chunk)]...)
		remaining -= chunk
	}
	return payload, nil
}

func decodeSQLiteSchemaObject(payload []byte, encoding uint32) (sqliteSchemaObject, error) {
	values, err := decodeSQLiteRecord(payload, encoding)
	if err != nil {
		return sqliteSchemaObject{}, err
	}
	if len(values) != 5 {
		return sqliteSchemaObject{}, fmt.Errorf("sqlite_schema record has %d fields, expected 5", len(values))
	}
	for _, index := range []int{0, 1, 2} {
		if values[index].kind != "text" {
			return sqliteSchemaObject{}, fmt.Errorf("sqlite_schema field %d is not text", index)
		}
	}
	if len(values[1].text) > maxSQLiteNameBytes || len(values[2].text) > maxSQLiteNameBytes {
		return sqliteSchemaObject{}, fmt.Errorf("sqlite_schema object name exceeds %d bytes", maxSQLiteNameBytes)
	}
	if !validSQLiteSchemaName(values[1].text) || !validSQLiteSchemaName(values[2].text) {
		return sqliteSchemaObject{}, errors.New("sqlite_schema object name contains invalid UTF-8, NUL, or control bytes")
	}
	rootPage := int64(0)
	if values[3].kind == "integer" {
		rootPage = values[3].integer
	} else if values[3].kind != "null" {
		return sqliteSchemaObject{}, errors.New("sqlite_schema root page is not an integer or NULL")
	}
	if rootPage < 0 || rootPage > math.MaxUint32 {
		return sqliteSchemaObject{}, fmt.Errorf("sqlite_schema root page %d is out of range", rootPage)
	}
	sqlText := ""
	sqlSeen := false
	if values[4].kind == "text" {
		sqlText = values[4].text
		sqlSeen = true
	} else if values[4].kind != "null" {
		return sqliteSchemaObject{}, errors.New("sqlite_schema SQL field is not text or NULL")
	}
	objectType := strings.ToLower(values[0].text)
	if objectType != "table" && objectType != "index" && objectType != "view" && objectType != "trigger" {
		return sqliteSchemaObject{}, fmt.Errorf("unsupported sqlite_schema object type %q", values[0].text)
	}
	normalizedSQL := strings.ToUpper(strings.TrimSpace(sqlText))
	return sqliteSchemaObject{
		objectType:    objectType,
		name:          values[1].text,
		tableName:     values[2].text,
		rootPage:      uint32(rootPage),
		sqlBytes:      len(sqlText),
		virtualTable:  strings.HasPrefix(normalizedSQL, "CREATE VIRTUAL TABLE"),
		withoutRowID:  strings.Contains(normalizedSQL, "WITHOUT ROWID"),
		internal:      strings.HasPrefix(strings.ToLower(values[1].text), "sqlite_"),
		schemaSQLSeen: sqlSeen,
	}, nil
}

func decodeSQLiteRecord(payload []byte, encoding uint32) ([]sqliteRecordValue, error) {
	headerSize, consumed, err := readSQLiteVarint(payload)
	if err != nil {
		return nil, fmt.Errorf("read record header size: %w", err)
	}
	if headerSize < uint64(consumed) || headerSize > uint64(len(payload)) {
		return nil, fmt.Errorf("record header size %d exceeds payload %d", headerSize, len(payload))
	}
	serialTypes := []uint64{}
	headerOffset := consumed
	for headerOffset < int(headerSize) {
		serialType, n, err := readSQLiteVarint(payload[headerOffset:int(headerSize)])
		if err != nil {
			return nil, fmt.Errorf("read record serial type: %w", err)
		}
		serialTypes = append(serialTypes, serialType)
		headerOffset += n
		if len(serialTypes) > 64 {
			return nil, errors.New("record has more than 64 fields")
		}
	}
	bodyOffset := int(headerSize)
	values := make([]sqliteRecordValue, 0, len(serialTypes))
	for _, serialType := range serialTypes {
		length, kind, err := sqliteSerialType(serialType)
		if err != nil {
			return nil, err
		}
		if length > len(payload)-bodyOffset {
			return nil, errors.New("record field exceeds payload bytes")
		}
		field := payload[bodyOffset : bodyOffset+length]
		bodyOffset += length
		value := sqliteRecordValue{kind: kind}
		switch kind {
		case "integer":
			value.integer = decodeSQLiteSignedInteger(field)
		case "zero":
			value.kind = "integer"
			value.integer = 0
		case "one":
			value.kind = "integer"
			value.integer = 1
		case "text":
			text, err := decodeSQLiteText(field, encoding)
			if err != nil {
				return nil, err
			}
			value.text = text
		case "float", "blob":
			// The schema table does not legitimately use these storage classes.
		}
		values = append(values, value)
	}
	return values, nil
}

func sqliteSerialType(serialType uint64) (int, string, error) {
	switch serialType {
	case 0:
		return 0, "null", nil
	case 1:
		return 1, "integer", nil
	case 2:
		return 2, "integer", nil
	case 3:
		return 3, "integer", nil
	case 4:
		return 4, "integer", nil
	case 5:
		return 6, "integer", nil
	case 6:
		return 8, "integer", nil
	case 7:
		return 8, "float", nil
	case 8:
		return 0, "zero", nil
	case 9:
		return 0, "one", nil
	case 10, 11:
		return 0, "", fmt.Errorf("reserved record serial type %d", serialType)
	default:
		length64 := (serialType - 12) / 2
		if length64 > maxSQLiteSchemaRecordBytes {
			return 0, "", fmt.Errorf("record field length %d exceeds limit", length64)
		}
		kind := "blob"
		if serialType&1 == 1 {
			kind = "text"
		}
		return int(length64), kind, nil
	}
}

func readSQLiteVarint(data []byte) (uint64, int, error) {
	if len(data) == 0 {
		return 0, 0, io.ErrUnexpectedEOF
	}
	var value uint64
	limit := len(data)
	if limit > 9 {
		limit = 9
	}
	for index := 0; index < limit; index++ {
		if index == 8 {
			value = value<<8 | uint64(data[index])
			return value, 9, nil
		}
		value = value<<7 | uint64(data[index]&0x7f)
		if data[index]&0x80 == 0 {
			return value, index + 1, nil
		}
	}
	return 0, 0, errors.New("truncated SQLite varint")
}

func sqliteTableLeafLocalPayload(payloadSize, usableSize uint64) uint64 {
	maximum := usableSize - 35
	if payloadSize <= maximum {
		return payloadSize
	}
	minimum := ((usableSize-12)*32)/255 - 23
	candidate := minimum + (payloadSize-minimum)%(usableSize-4)
	if candidate <= maximum {
		return candidate
	}
	return minimum
}

func decodeSQLiteSignedInteger(data []byte) int64 {
	if len(data) == 0 {
		return 0
	}
	var value uint64
	for _, item := range data {
		value = value<<8 | uint64(item)
	}
	if data[0]&0x80 != 0 && len(data) < 8 {
		value |= ^uint64(0) << (uint(len(data)) * 8)
	}
	return int64(value)
}

func decodeSQLiteText(data []byte, encoding uint32) (string, error) {
	switch encoding {
	case 0, 1:
		if !utf8.Valid(data) {
			return "", errors.New("schema text is not valid UTF-8")
		}
		return string(data), nil
	case 2, 3:
		if len(data)%2 != 0 {
			return "", errors.New("schema UTF-16 text has an odd byte length")
		}
		units := make([]uint16, len(data)/2)
		for index := range units {
			if encoding == 2 {
				units[index] = binary.LittleEndian.Uint16(data[index*2 : index*2+2])
			} else {
				units[index] = binary.BigEndian.Uint16(data[index*2 : index*2+2])
			}
		}
		decoded := string(utf16.Decode(units))
		if !utf8.ValidString(decoded) {
			return "", errors.New("schema UTF-16 text is invalid")
		}
		return decoded, nil
	default:
		return "", fmt.Errorf("unsupported schema text encoding %d", encoding)
	}
}

func validSQLiteSchemaName(value string) bool {
	if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		return false
	}
	for _, item := range value {
		if item < 0x20 && item != '\t' && item != '\n' && item != '\r' {
			return false
		}
	}
	return true
}

func sqliteEncodingName(value uint32) string {
	switch value {
	case 1:
		return "UTF-8"
	case 2:
		return "UTF-16le"
	case 3:
		return "UTF-16be"
	default:
		return "unspecified"
	}
}

func sqliteVersionString(value uint32) string {
	if value == 0 {
		return "unknown"
	}
	return fmt.Sprintf("%d.%d.%d", value/1_000_000, (value/1_000)%1_000, value%1_000)
}

func allZero(data []byte) bool {
	for _, value := range data {
		if value != 0 {
			return false
		}
	}
	return true
}
