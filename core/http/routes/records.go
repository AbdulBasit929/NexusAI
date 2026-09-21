package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/mudler/LocalAI/core/application"
	"github.com/mudler/LocalAI/core/http/endpoints/localai"
)

func RegisterRecordRoutes(e *echo.Echo, app *application.Application, recordsMw echo.MiddlewareFunc) {
	g := e.Group("/api/records", recordsMw)
	g.POST("/ingest", localai.IngestRecordsEndpoint(app))
	g.GET("/batches", localai.ListRecordBatchesEndpoint(app))
	g.GET("/batches/:id", localai.GetRecordBatchEndpoint(app))
	g.DELETE("/batches/:id", localai.DeleteRecordBatchEndpoint(app))
	g.POST("/query", localai.QueryRecordsEndpoint(app))
	g.POST("/aggregate", localai.AggregateRecordsEndpoint(app))
	g.POST("/correlate", localai.CorrelateRecordsEndpoint(app))
	g.GET("/schema/:record_type", localai.GetRecordSchemaEndpoint(app))
	g.GET("/forensic/status", localai.GetForensicRecordsStatusEndpoint(app))
	g.GET("/forensic/templates", localai.GetForensicRecordsTemplatesEndpoint())
	g.GET("/forensic/capabilities", localai.GetForensicRecordsCapabilitiesEndpoint(app))
	g.GET("/forensic/evidence", localai.ListForensicEvidenceEndpoint(app))
	g.GET("/forensic/evidence/:id", localai.GetForensicEvidenceEndpoint(app))
	g.POST("/forensic/evidence/:id/reprocess", localai.ReprocessForensicEvidenceEndpoint(app))
	g.POST("/forensic/query", localai.QueryForensicRecordsEndpoint(app))

	v1 := e.Group("/api/v1/forensics", recordsMw)
	v1.GET("/adapters", localai.ListForensicAdaptersV1Endpoint())
	v1.GET("/operations", localai.ListForensicOperationsV1Endpoint())
	v1.GET("/agents", localai.ListForensicSpecialistsV1Endpoint())
	v1.GET("/contracts", localai.ListForensicContractsV1Endpoint())
	v1.GET("/cases", localai.ListForensicCasesV1Endpoint(app))
	v1.GET("/cases/:case_id", localai.GetForensicCaseV1Endpoint(app))
	v1.GET("/cases/:case_id/manifest", localai.GetForensicCaseManifestV1Endpoint(app))
	v1.GET("/cases/:case_id/evidence", localai.ListForensicCaseEvidenceV1Endpoint(app))
	v1.GET("/cases/:case_id/evidence/compare", localai.CompareForensicCaseImagesV1Endpoint(app))
	v1.GET("/cases/:case_id/evidence/:evidence_id", localai.GetForensicCaseEvidenceV1Endpoint(app))
	v1.GET("/cases/:case_id/evidence/:evidence_id/content", localai.GetForensicCaseEvidenceContentV1Endpoint(app))
	v1.GET("/cases/:case_id/evidence/:evidence_id/reprocess-plan", localai.GetForensicCaseEvidenceReprocessPlanV1Endpoint(app))
	v1.GET("/cases/:case_id/faces/similar", localai.GetForensicCaseFaceSimilarityV1Endpoint(app))
	v1.GET("/cases/:case_id/images/similar", localai.GetForensicCaseImageSimilarityV1Endpoint(app))
	v1.POST("/cases/:case_id/query", localai.QueryForensicCaseV1Endpoint(app))
	v1.POST("/cases/:case_id/reports", localai.GenerateForensicCaseReportV1Endpoint(app))
}
