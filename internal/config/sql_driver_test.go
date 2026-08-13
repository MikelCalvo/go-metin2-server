package config

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
)

const configTestSQLDriverName = "go_metin2_config_driver_test"

var configTestSQLDriverOnce sync.Once

func registerConfigTestSQLDriver() {
	configTestSQLDriverOnce.Do(func() {
		sql.Register(configTestSQLDriverName, configTestSQLDriver{})
	})
}

type configTestSQLDriver struct{}

func (configTestSQLDriver) Open(string) (driver.Conn, error) {
	return configTestSQLConn{}, nil
}

type configTestSQLConn struct{}

func (configTestSQLConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare unsupported by config test driver")
}

func (configTestSQLConn) Close() error {
	return nil
}

func (configTestSQLConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions unsupported by config test driver")
}
