package records

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func fixturePath(name string) string {
	return filepath.Join("..", "..", "..", "tests", "fixtures", "records", name)
}

func readFixture(name string) []byte {
	data, err := os.ReadFile(fixturePath(name))
	Expect(err).NotTo(HaveOccurred())
	return data
}

var _ = Describe("records parsers", func() {
	It("parses CSV and infers CDR schema", func() {
		parsed, err := Parse("cdr_sample.csv", strings.NewReader(string(readFixture("cdr_sample.csv"))))
		Expect(err).NotTo(HaveOccurred())
		Expect(parsed.Rows).To(HaveLen(4))
		Expect(parsed.RecordType).To(Equal(RecordTypeCDR))
		Expect(parsed.Fields).To(ContainElement(WithTransform(func(f FieldSchema) string {
			return f.CanonicalName
		}, Equal("duration_seconds"))))
	})

	It("parses TSV records", func() {
		parsed, err := Parse("sample.tsv", strings.NewReader("plate_number\tlocation\nLEA-123\tGulberg\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(parsed.Rows).To(HaveLen(1))
		Expect(parsed.RecordType).To(Equal(RecordTypeANPR))
	})

	It("parses JSONL and JSON arrays", func() {
		jsonl, err := Parse("ipdr_sample.jsonl", strings.NewReader(string(readFixture("ipdr_sample.jsonl"))))
		Expect(err).NotTo(HaveOccurred())
		Expect(jsonl.Rows).To(HaveLen(2))
		Expect(jsonl.RecordType).To(Equal(RecordTypeIPDR))

		jsonArray, err := Parse("records.json", strings.NewReader(`[{"transaction_id":"txn-1","amount":10},{"transaction_id":"txn-2","amount":20}]`))
		Expect(err).NotTo(HaveOccurred())
		Expect(jsonArray.Rows).To(HaveLen(2))
		Expect(jsonArray.RecordType).To(Equal(RecordTypeTransaction))
	})
})

var _ = Describe("records service", func() {
	var (
		dir string
		svc *Service
	)

	BeforeEach(func() {
		var err error
		dir, err = os.MkdirTemp("", "localai-records-*")
		Expect(err).NotTo(HaveOccurred())
		svc = NewService(dir)
	})

	AfterEach(func() {
		_ = os.RemoveAll(dir)
	})

	ingest := func(name, collection, recordType string) IngestResult {
		result, err := svc.Ingest(readFixture(name), IngestOptions{
			CollectionName: collection,
			RecordType:     recordType,
			SourceFile:     name,
			SourceEntry:    "source/" + name,
		})
		Expect(err).NotTo(HaveOccurred())
		return result
	}

	It("runs deterministic CDR helper queries", func() {
		batch := ingest("cdr_sample.csv", "case-a", "")
		Expect(batch.Batch.RecordType).To(Equal(RecordTypeCDR))

		shortest, err := svc.Query("", QueryRequest{BatchIDs: []string{batch.Batch.ID}, Helper: "shortest_call"})
		Expect(err).NotTo(HaveOccurred())
		Expect(shortest.Records).To(HaveLen(1))
		Expect(shortest.Records[0]["call_id"]).To(Equal("cdr-004"))

		longest, err := svc.Query("", QueryRequest{BatchIDs: []string{batch.Batch.ID}, Helper: "longest_call"})
		Expect(err).NotTo(HaveOccurred())
		Expect(longest.Records[0]["call_id"]).To(Equal("cdr-003"))

		failed, err := svc.Query("", QueryRequest{BatchIDs: []string{batch.Batch.ID}, Helper: "failed_calls"})
		Expect(err).NotTo(HaveOccurred())
		Expect(failed.Records).To(HaveLen(1))
		Expect(failed.Records[0]["call_id"]).To(Equal("cdr-002"))
	})

	It("filters, aggregates, and returns source metadata", func() {
		batch := ingest("cdr_sample.csv", "case-a", "")
		query, err := svc.Query("", QueryRequest{
			BatchIDs: []string{batch.Batch.ID},
			Filters:  []Filter{{Field: "area", Op: "eq", Value: "Gulberg"}},
			Sort:     []SortField{{Field: "duration_seconds", Direction: "asc"}},
			Limit:    10,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(query.Total).To(Equal(3))
		Expect(query.Records[0]["source_entry"]).To(Equal("source/cdr_sample.csv"))

		agg, err := svc.Aggregate("", AggregateRequest{
			QueryRequest: QueryRequest{BatchIDs: []string{batch.Batch.ID}},
			Operation:    "top_by_field",
			Field:        "target_number",
			TopN:         1,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(agg.Results).To(HaveLen(1))
		Expect(agg.Results[0].Value).To(Equal("03111234567"))
		Expect(agg.Results[0].Count).To(Equal(2))
	})

	It("runs ANPR helper queries", func() {
		batch := ingest("anpr_sample.csv", "case-a", "")
		resp, err := svc.Query("", QueryRequest{
			BatchIDs: []string{batch.Batch.ID},
			Helper:   "repeat_plate_locations",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Records).To(ContainElement(HaveKeyWithValue("plate_number", "LEA-123")))
		Expect(resp.Records).To(ContainElement(HaveKeyWithValue("location", "Gulberg Main")))
	})

	It("correlates records by shared entity values and time window", func() {
		cdr := ingest("cdr_sample.csv", "case-a", "")
		ipdr := ingest("ipdr_sample.jsonl", "case-a", "")
		resp, err := svc.Correlate("", CorrelateRequest{
			Left:              QueryRequest{BatchIDs: []string{cdr.Batch.ID}},
			Right:             QueryRequest{BatchIDs: []string{ipdr.Batch.ID}},
			EntityFields:      []string{"source_number", "msisdn"},
			TimeWindowSeconds: 120,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Count).To(BeNumerically(">=", 1))
		Expect(resp.Matches[0].Entity["value"]).To(Equal("03001234567"))
	})
})
