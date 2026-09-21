package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type sqliteTestSchemaObject struct {
	objectType string
	name       string
	tableName  string
	rootPage   byte
	sql        string
}

var _ = Describe("Deterministic SQLite inventory", func() {
	It("inventories header, schema objects, and bounded table row counts without executing SQL", func() {
		path := filepath.Join(GinkgoT().TempDir(), "device.sqlite")
		Expect(os.WriteFile(path, sqliteTestDatabase(false), 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "device.sqlite", evidenceClassification{Modality: "database"})
		Expect(warnings).To(BeEmpty())
		Expect(metadata).To(HaveKeyWithValue("format", "sqlite3"))
		Expect(metadata).To(HaveKeyWithValue("extractor_version", "1.5.0"))
		Expect(metadata).To(HaveKeyWithValue("inspection_mode", "raw_file_read_only"))
		Expect(metadata).To(HaveKeyWithValue("page_size_bytes", 4096))
		Expect(metadata).To(HaveKeyWithValue("actual_page_count", uint32(3)))
		Expect(metadata).To(HaveKeyWithValue("declared_page_count", uint32(3)))
		Expect(metadata).To(HaveKeyWithValue("schema_format", uint32(4)))
		Expect(metadata).To(HaveKeyWithValue("text_encoding", "UTF-8"))
		Expect(metadata).To(HaveKeyWithValue("write_library_version", "3.46.0"))
		Expect(metadata).To(HaveKeyWithValue("schema_object_count", 2))
		Expect(metadata).To(HaveKeyWithValue("table_count", 1))
		Expect(metadata).To(HaveKeyWithValue("index_count", 1))
		Expect(metadata).To(HaveKeyWithValue("inventory_complete", true))
		Expect(metadata).To(HaveKeyWithValue("table_row_counts_complete", true))
		Expect(metadata).To(HaveKeyWithValue("sql_executed", false))
		Expect(metadata).To(HaveKeyWithValue("extensions_loaded", false))
		Expect(metadata).To(HaveKeyWithValue("sidecar_files_opened", false))
		Expect(metadata).To(HaveKeyWithValue("data_values_read", false))
		Expect(metadata).To(HaveKeyWithValue("schema_sql_retained", false))

		objects := metadata["schema_objects"].([]map[string]any)
		Expect(objects).To(ContainElement(And(
			HaveKeyWithValue("type", "table"),
			HaveKeyWithValue("name", "calls"),
			HaveKeyWithValue("root_page", uint32(2)),
			HaveKeyWithValue("sql_bytes", BeNumerically(">", 0)),
		)))
		Expect(objects[0]).NotTo(HaveKey("sql"))
		tables := metadata["table_summaries"].([]map[string]any)
		Expect(tables).To(ConsistOf(And(
			HaveKeyWithValue("name", "calls"),
			HaveKeyWithValue("row_count", uint64(3)),
			HaveKeyWithValue("count_complete", true),
		)))
	})

	It("warns that a WAL-mode main file cannot represent omitted sidecar frames", func() {
		path := filepath.Join(GinkgoT().TempDir(), "wal.db")
		Expect(os.WriteFile(path, sqliteTestDatabase(true), 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "wal.db", evidenceClassification{Modality: "database"})
		Expect(metadata).To(HaveKeyWithValue("wal_mode_header", true))
		Expect(metadata).To(HaveKeyWithValue("sidecar_files_opened", false))
		Expect(warnings).To(ContainElement(ContainSubstring("uncheckpointed frames")))
	})

	It("inventories trigger and virtual-table definitions without executing them", func() {
		objects := []sqliteTestSchemaObject{
			{objectType: "table", name: "calls", tableName: "calls", rootPage: 2, sql: "CREATE TABLE calls(number TEXT)"},
			{objectType: "table", name: "messages_fts", tableName: "messages_fts", sql: "CREATE VIRTUAL TABLE messages_fts USING fts5(body)"},
			{objectType: "trigger", name: "calls_audit", tableName: "calls", sql: "CREATE TRIGGER calls_audit AFTER INSERT ON calls BEGIN SELECT 1; END"},
		}
		payload := sqliteTestDatabaseWithObjects(objects)
		path := filepath.Join(GinkgoT().TempDir(), "active-schema.sqlite3")
		Expect(os.WriteFile(path, payload, 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "active-schema.sqlite3", evidenceClassification{Modality: "database"})
		Expect(metadata).To(HaveKeyWithValue("virtual_table_count", 1))
		Expect(metadata).To(HaveKeyWithValue("trigger_count", 1))
		Expect(metadata).To(HaveKeyWithValue("sql_executed", false))
		Expect(warnings).To(ContainElement(ContainSubstring("never executed")))
	})

	It("preserves partial inventory when a table b-tree contains a cycle", func() {
		payload := sqliteTestDatabase(false)
		pageTwo := payload[4096:8192]
		for index := range pageTwo {
			pageTwo[index] = 0
		}
		pageTwo[0] = 0x05
		binary.BigEndian.PutUint16(pageTwo[5:7], 4096)
		binary.BigEndian.PutUint32(pageTwo[8:12], 2)
		path := filepath.Join(GinkgoT().TempDir(), "cyclic-table.sqlite")
		Expect(os.WriteFile(path, payload, 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "cyclic-table.sqlite", evidenceClassification{Modality: "database"})
		Expect(metadata).To(HaveKeyWithValue("schema_object_count", 2))
		Expect(metadata).To(HaveKeyWithValue("table_row_counts_complete", false))
		Expect(metadata).To(HaveKeyWithValue("inventory_complete", false))
		Expect(warnings).To(ContainElement(ContainSubstring("page cycle")))
	})

	It("rejects corrupt headers, impossible page arrays, and cyclic sqlite_schema trees", func() {
		valid := sqliteTestDatabase(false)
		badMagic := append([]byte{}, valid...)
		badMagic[0] = 'X'
		badPointers := append([]byte{}, valid...)
		binary.BigEndian.PutUint16(badPointers[103:105], 65535)
		cyclicSchema := append([]byte{}, valid...)
		for index := 100; index < 4096; index++ {
			cyclicSchema[index] = 0
		}
		cyclicSchema[100] = 0x05
		binary.BigEndian.PutUint16(cyclicSchema[105:107], 4096)
		binary.BigEndian.PutUint32(cyclicSchema[108:112], 1)

		for _, fixture := range []struct {
			name    string
			payload []byte
			warning string
		}{
			{name: "bad-magic.db", payload: badMagic, warning: "missing SQLite format 3 header"},
			{name: "bad-pointers.db", payload: badPointers, warning: "cell pointer array"},
			{name: "cyclic-schema.db", payload: cyclicSchema, warning: "page cycle"},
		} {
			path := filepath.Join(GinkgoT().TempDir(), fixture.name)
			Expect(os.WriteFile(path, fixture.payload, 0o600)).To(Succeed())
			metadata, warnings := extractDeterministicMediaMetadata(path, fixture.name, evidenceClassification{Modality: "database"})
			Expect(metadata).To(BeEmpty())
			Expect(warnings).To(ConsistOf(ContainSubstring(fixture.warning)))
		}
	})

	It("marks page-count disagreement incomplete and never parses SQL dumps automatically", func() {
		payload := sqliteTestDatabase(false)
		binary.BigEndian.PutUint32(payload[28:32], 2)
		path := filepath.Join(GinkgoT().TempDir(), "mismatch.sqlite")
		Expect(os.WriteFile(path, payload, 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "mismatch.sqlite", evidenceClassification{Modality: "database"})
		Expect(metadata).To(HaveKeyWithValue("inventory_complete", false))
		Expect(warnings).To(ContainElement(ContainSubstring("declares 2 pages")))

		sqlPath := filepath.Join(GinkgoT().TempDir(), "dump.sql")
		Expect(os.WriteFile(sqlPath, []byte("CREATE TABLE evidence(id INTEGER);"), 0o600)).To(Succeed())
		metadata, warnings = extractDeterministicMediaMetadata(sqlPath, "dump.sql", evidenceClassification{Modality: "database"})
		Expect(metadata).To(BeEmpty())
		Expect(warnings).To(BeEmpty())
	})

	It("bounds schema object names before retaining them", func() {
		payload := sqliteTestDatabaseWithObjects([]sqliteTestSchemaObject{{
			objectType: "table",
			name:       strings.Repeat("n", maxSQLiteNameBytes+1),
			tableName:  "calls",
			rootPage:   2,
			sql:        "CREATE TABLE calls(number TEXT)",
		}})
		path := filepath.Join(GinkgoT().TempDir(), "long-name.sqlite")
		Expect(os.WriteFile(path, payload, 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "long-name.sqlite", evidenceClassification{Modality: "database"})
		Expect(metadata).To(BeEmpty())
		Expect(warnings).To(ConsistOf(ContainSubstring("object name exceeds 512 bytes")))
	})
})

func sqliteTestDatabase(walMode bool) []byte {
	return sqliteTestDatabaseWithObjectsAndMode([]sqliteTestSchemaObject{
		{objectType: "table", name: "calls", tableName: "calls", rootPage: 2, sql: "CREATE TABLE calls(number TEXT, observed_at TEXT)"},
		{objectType: "index", name: "idx_calls_number", tableName: "calls", rootPage: 3, sql: "CREATE INDEX idx_calls_number ON calls(number)"},
	}, walMode)
}

func sqliteTestDatabaseWithObjects(objects []sqliteTestSchemaObject) []byte {
	return sqliteTestDatabaseWithObjectsAndMode(objects, false)
}

func sqliteTestDatabaseWithObjectsAndMode(objects []sqliteTestSchemaObject, walMode bool) []byte {
	const pageSize = 4096
	payload := make([]byte, pageSize*3)
	copy(payload[:16], []byte("SQLite format 3\x00"))
	binary.BigEndian.PutUint16(payload[16:18], pageSize)
	version := byte(1)
	if walMode {
		version = 2
	}
	payload[18] = version
	payload[19] = version
	payload[20] = 0
	payload[21] = 64
	payload[22] = 32
	payload[23] = 32
	binary.BigEndian.PutUint32(payload[24:28], 1)
	binary.BigEndian.PutUint32(payload[28:32], 3)
	binary.BigEndian.PutUint32(payload[40:44], 1)
	binary.BigEndian.PutUint32(payload[44:48], 4)
	binary.BigEndian.PutUint32(payload[56:60], 1)
	binary.BigEndian.PutUint32(payload[92:96], 1)
	binary.BigEndian.PutUint32(payload[96:100], 3_046_000)

	schemaCells := make([][]byte, 0, len(objects))
	for index, object := range objects {
		record := sqliteTestSchemaRecord(object)
		cell := append(sqliteTestVarint(uint64(len(record))), sqliteTestVarint(uint64(index+1))...)
		cell = append(cell, record...)
		schemaCells = append(schemaCells, cell)
	}
	sqliteTestLeafPage(payload[:pageSize], 100, 0x0d, schemaCells)

	rowCells := make([][]byte, 0, 3)
	for rowID := 1; rowID <= 3; rowID++ {
		record := []byte{2, 0}
		cell := append(sqliteTestVarint(uint64(len(record))), sqliteTestVarint(uint64(rowID))...)
		cell = append(cell, record...)
		rowCells = append(rowCells, cell)
	}
	sqliteTestLeafPage(payload[pageSize:pageSize*2], 0, 0x0d, rowCells)
	sqliteTestLeafPage(payload[pageSize*2:], 0, 0x0a, nil)
	return payload
}

func sqliteTestLeafPage(page []byte, headerOffset int, pageType byte, cells [][]byte) {
	page[headerOffset] = pageType
	binary.BigEndian.PutUint16(page[headerOffset+3:headerOffset+5], uint16(len(cells)))
	contentOffset := len(page)
	for index, cell := range cells {
		contentOffset -= len(cell)
		copy(page[contentOffset:], cell)
		pointerOffset := headerOffset + 8 + index*2
		binary.BigEndian.PutUint16(page[pointerOffset:pointerOffset+2], uint16(contentOffset))
	}
	binary.BigEndian.PutUint16(page[headerOffset+5:headerOffset+7], uint16(contentOffset))
}

func sqliteTestSchemaRecord(object sqliteTestSchemaObject) []byte {
	texts := []string{object.objectType, object.name, object.tableName}
	serialTypes := make([]uint64, 0, 5)
	body := []byte{}
	for _, value := range texts {
		serialTypes = append(serialTypes, uint64(13+len(value)*2))
		body = append(body, []byte(value)...)
	}
	if object.rootPage == 0 {
		serialTypes = append(serialTypes, 8)
	} else {
		serialTypes = append(serialTypes, 1)
		body = append(body, object.rootPage)
	}
	if object.sql == "" {
		serialTypes = append(serialTypes, 0)
	} else {
		serialTypes = append(serialTypes, uint64(13+len(object.sql)*2))
		body = append(body, []byte(object.sql)...)
	}

	serialHeader := []byte{}
	for _, serialType := range serialTypes {
		serialHeader = append(serialHeader, sqliteTestVarint(serialType)...)
	}
	headerSize := len(serialHeader) + 1
	for {
		encodedSize := sqliteTestVarint(uint64(headerSize))
		next := len(serialHeader) + len(encodedSize)
		if next == headerSize {
			return append(append(encodedSize, serialHeader...), body...)
		}
		headerSize = next
	}
}

func sqliteTestVarint(value uint64) []byte {
	if value <= 0x7f {
		return []byte{byte(value)}
	}
	groups := []byte{byte(value & 0x7f)}
	value >>= 7
	for value > 0 {
		groups = append(groups, byte(value&0x7f)|0x80)
		value >>= 7
	}
	encoded := make([]byte, len(groups))
	for index := range groups {
		encoded[index] = groups[len(groups)-1-index]
	}
	encoded[len(encoded)-1] &= 0x7f
	return encoded
}
