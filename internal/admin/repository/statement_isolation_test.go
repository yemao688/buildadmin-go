package repository_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	model "buildadmin-go/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// newDryRunDB builds an offline GORM handle: statements are built and logged
// without opening a MySQL connection, so these tests need no database. The
// naming strategy mirrors internal/infra/db so model-inferred table names match
// production (ba_country_currency, not country_currencies).
func newDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		mysql.New(mysql.Config{DSN: "user:pass@tcp(127.0.0.1:3306)/unused", SkipInitializeWithVersion: true}),
		&gorm.Config{
			DryRun:               true,
			DisableAutomaticPing: true,
			NamingStrategy:       schema.NamingStrategy{TablePrefix: "ba_", SingularTable: true},
		},
	)
	require.NoError(t, err)
	return db
}

func newRepositoryTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return ctx
}

type queryCapture struct {
	sqls []string
}

func (c *queryCapture) reset() { c.sqls = nil }

func (c *queryCapture) last(t *testing.T) string {
	t.Helper()
	require.NotEmpty(t, c.sqls, "expected a captured query")
	return c.sqls[len(c.sqls)-1]
}

// captureQueries records the SQL of every query callback, which keeps the
// generated repositories' own execution handles observable.
func captureQueries(db *gorm.DB) *queryCapture {
	capture := &queryCapture{}
	err := db.Callback().Query().After("gorm:query").Register("test:capture_queries", func(tx *gorm.DB) {
		capture.sqls = append(capture.sqls, tx.Statement.SQL.String())
	})
	if err != nil {
		panic(err)
	}
	return capture
}

func newCountryRepositories(t *testing.T, db *gorm.DB) (*repository.CountryLanguageRepository, *repository.CountryCurrencyRepository) {
	t.Helper()
	cfg := &conf.Configuration{}
	cfg.Database.Prefix = "ba_"
	return repository.NewCountryLanguageRepository(db, cfg, nil), repository.NewCountryCurrencyRepository(db, cfg, nil)
}

// TestGeneratedRepositoryReadsKeepSharedStatementClean is the regression test
// for issue #9: generated GetOne/List used to write db.Statement.Table on a
// session that shares its statement pointer with the long-lived connection,
// so every later query on that handle targeted the generated table instead of
// the one inferred from its model. See internal/pkg/crud_helper/defined.go.
func TestGeneratedRepositoryReadsKeepSharedStatementClean(t *testing.T) {
	db := newDryRunDB(t)
	ctx := newRepositoryTestContext(t)
	language, currency := newCountryRepositories(t, db)
	capture := captureQueries(db)

	_, err := language.GetOne(ctx, 1)
	require.NoError(t, err)
	require.Contains(t, capture.last(t), "`ba_country_language`")

	_, _, err = language.List(ctx)
	require.NoError(t, err)
	require.Contains(t, capture.last(t), "`ba_country_language`")

	require.Empty(t, db.Statement.Table, "generated reads must not write the shared statement")

	// A second generated repository must still read its own table.
	capture.reset()
	_, err = currency.GetOne(ctx, 1)
	require.NoError(t, err)
	require.Contains(t, capture.last(t), "`ba_country_currency`")

	// Model-inferred queries go through GORM's Session/WithContext clone paths,
	// which inherit whatever Table the shared statement carries.
	capture.reset()
	var row model.CountryCurrency
	require.NoError(t, db.WithContext(context.Background()).Model(&model.CountryCurrency{}).Where("code = ?", "USD").Take(&row).Error)
	require.Contains(t, capture.last(t), "`ba_country_currency`")
	require.NotContains(t, capture.last(t), "ba_country_language")
}

// TestGeneratedRepositoryReadsKeepRequestTransactionStatementClean covers the
// request-transaction handle bound by the admin middleware
// (tx.WithContext(request context), a session that shares its statement with
// every repository read in the request).
func TestGeneratedRepositoryReadsKeepRequestTransactionStatementClean(t *testing.T) {
	db := newDryRunDB(t)
	language, _ := newCountryRepositories(t, db)

	requestDB := db.WithContext(context.Background())
	ctx := contextWithRequestTransaction(t, requestDB)

	_, err := language.GetOne(ctx, 1)
	require.NoError(t, err)
	_, _, err = language.List(ctx)
	require.NoError(t, err)

	require.Empty(t, requestDB.Statement.Table, "generated reads must not write the request transaction statement")

	var row model.CountryCurrency
	q := requestDB.Model(&model.CountryCurrency{}).Where("code = ?", "USD").Take(&row)
	require.NoError(t, q.Error)
	require.Contains(t, q.Statement.SQL.String(), "`ba_country_currency`")
	require.NotContains(t, q.Statement.SQL.String(), "ba_country_language")
}
