package testutil

import (
	"context"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// SetupTestDB creates a fresh PostgreSQL 16 container and returns a gorm.DB.
// The container is cleaned up via DeferCleanup when the test completes.
func SetupTestDB() *gorm.DB {
	if connStr := os.Getenv("LOCALAI_TEST_DATABASE_URL"); connStr != "" {
		parsed, err := url.Parse(connStr)
		Expect(err).NotTo(HaveOccurred())
		Expect(parsed.Path).To(Equal("/nxmmr_p1_test"), "external test DB must be explicitly isolated")
		admin, err := gorm.Open(postgres.Open(connStr), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		Expect(err).NotTo(HaveOccurred())
		var database string
		Expect(admin.Raw("SELECT current_database()").Scan(&database).Error).To(Succeed())
		Expect(database).To(Equal("nxmmr_p1_test"))
		schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		Expect(admin.Exec("CREATE SCHEMA " + schema).Error).To(Succeed())
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			sqlDB, _ := db.DB()
			_ = sqlDB.Close()
			Expect(admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error).To(Succeed())
			adminDB, _ := admin.DB()
			_ = adminDB.Close()
		})
		return db
	}
	if runtime.GOOS == "darwin" {
		Skip("testcontainers requires Docker, not available on macOS CI")
	}
	ctx := context.Background()
	pgC, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategyAndDeadline(60*time.Second,
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
	)
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(func() { pgC.Terminate(context.Background()) })
	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	Expect(err).ToNot(HaveOccurred())
	db, err := gorm.Open(postgres.Open(connStr), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	Expect(err).ToNot(HaveOccurred())
	return db
}
