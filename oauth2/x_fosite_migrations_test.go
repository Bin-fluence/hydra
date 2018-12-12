package oauth2_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/ory/fosite"
	"github.com/ory/hydra/client"
	"github.com/ory/hydra/consent"
	"github.com/ory/hydra/oauth2"
	"github.com/ory/x/dbal"
	"github.com/ory/x/dbal/migratest"
)

var createMigrations = map[string]*dbal.PackrMigrationSource{
	dbal.DriverMySQL:      dbal.NewMustPackerMigrationSource(logrus.New(), oauth2.AssetNames(), oauth2.Asset, []string{"migrations/sql/tests"}, true),
	dbal.DriverPostgreSQL: dbal.NewMustPackerMigrationSource(logrus.New(), oauth2.AssetNames(), oauth2.Asset, []string{"migrations/sql/tests"}, true),
}

func cleanDB(t *testing.T, db *sqlx.DB) {
	t.Logf("Cleaning up tables...")

	_, _ = db.Exec("DROP TABLE IF EXISTS hydra_oauth2_access")
	_, _ = db.Exec("DROP TABLE IF EXISTS hydra_oauth2_refresh")
	_, _ = db.Exec("DROP TABLE IF EXISTS hydra_oauth2_code")
	_, _ = db.Exec("DROP TABLE IF EXISTS hydra_oauth2_oidc")
	_, _ = db.Exec("DROP TABLE IF EXISTS hydra_oauth2_pkce")

	// hydra_oauth2_consent_request_handled depends on hydra_oauth2_consent_request
	_, _ = db.Exec("DROP TABLE IF EXISTS hydra_oauth2_consent_request_handled")
	_, _ = db.Exec("DROP TABLE IF EXISTS hydra_oauth2_consent_request")

	// hydra_oauth2_authentication_request_handled depends on hydra_oauth2_authentication_request
	_, _ = db.Exec("DROP TABLE IF EXISTS hydra_oauth2_authentication_request_handled")
	_, _ = db.Exec("DROP TABLE IF EXISTS hydra_oauth2_authentication_request")

	// everything depends on hydra_oauth2_authentication_session
	_, _ = db.Exec("DROP TABLE IF EXISTS hydra_oauth2_authentication_session")
	_, _ = db.Exec("DROP TABLE IF EXISTS hydra_oauth2_obfuscated_authentication_session")
	_, _ = db.Exec("DROP TABLE IF EXISTS hydra_client")

	// clean up migration tables
	_, _ = db.Exec("DROP TABLE IF EXISTS hydra_oauth2_authentication_consent_migration")
	_, _ = db.Exec("DROP TABLE IF EXISTS hydra_client_migration")
	_, _ = db.Exec("DROP TABLE IF EXISTS hydra_oauth2_migration")

	t.Logf("Done cleaning up tables!")
}

func TestXXMigrations(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}

	migratest.RunPackrMigrationTests(
		t,
		migratest.MigrationSchemas{client.Migrations, consent.Migrations, oauth2.Migrations},
		migratest.MigrationSchemas{nil, nil, createMigrations},
		cleanDB, cleanDB,
		func(t *testing.T, db *sqlx.DB, m, k, steps int) {
			t.Run(fmt.Sprintf("poll=%d", k), func(t *testing.T) {
				if m != 2 {
					t.Skip("Skipping polling unless it's the last migration schema")
					return
				}

				cm := client.NewSQLManager(db, &fosite.BCrypt{WorkFactor: 4})
				s := oauth2.NewFositeSQLStore(cm, db, logrus.New(), time.Minute, false)
				sig := fmt.Sprintf("%d-sig", k+1)

				if k < 8 {
					// With migration 8, all previous test data has been removed because the client is non-existent.
					_, err := s.GetAccessTokenSession(context.Background(), sig, oauth2.NewSession(""))
					require.Error(t, err)
					return
				}

				_, err := s.GetAccessTokenSession(context.Background(), sig, oauth2.NewSession(""))
				require.NoError(t, err)
				_, err = s.GetRefreshTokenSession(context.Background(), sig, oauth2.NewSession(""))
				require.NoError(t, err)
				_, err = s.GetAuthorizeCodeSession(context.Background(), sig, oauth2.NewSession(""))
				require.NoError(t, err)
				_, err = s.GetOpenIDConnectSession(context.Background(), sig, &fosite.Request{Session: oauth2.NewSession("")})
				require.NoError(t, err)
				if k > 2 {
					_, err = s.GetPKCERequestSession(context.Background(), sig, oauth2.NewSession(""))
					require.NoError(t, err)
				}
			})
		},
	)
}