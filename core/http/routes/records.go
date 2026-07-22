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
	g.GET("/forensic/status", localai.GetForensicRecordsStatusEndpoint())
	g.GET("/forensic/templates", localai.GetForensicRecordsTemplatesEndpoint())
	g.GET("/forensic/evidence", localai.ListForensicEvidenceEndpoint())
	g.GET("/forensic/evidence/:id", localai.GetForensicEvidenceEndpoint())
	g.POST("/forensic/query", localai.QueryForensicRecordsEndpoint())
}
