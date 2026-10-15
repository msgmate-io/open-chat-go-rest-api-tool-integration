package restapitoolintegration

import (
	"errors"
	"net/http"

	"github.com/msgmate-io/go-integration-interface/integrationinterface"
	"gorm.io/gorm"
)

var errHostNotInstalled = errors.New("rest api tool integration: host is not installed")

// dbAndUser resolves the request-scoped database handle and authenticated user
// through the stable Host surface. It is the counterpart of
// backend/server/util GetDBAndUser and replaces the previous direct import.
func dbAndUser(r *http.Request) (*gorm.DB, integrationinterface.User, error) {
	host := integrationinterface.GetHost()
	if host == nil {
		return nil, nil, errHostNotInstalled
	}
	storage, user, err := host.HTTP().DBAndUser(r)
	if err != nil {
		return nil, nil, err
	}
	db, _ := storage.Handle().(*gorm.DB)
	return db, user, nil
}
