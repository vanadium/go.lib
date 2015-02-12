// Utility functions for opening and configuring a connection to a MySQL-like
// database, with optional TLS support.
//
// Functions in this file are not thread-safe. However, the returned *sql.DB is.
// Sane defaults are assumed: utf8mb4 encoding, UTC timezone, parsing date/time
// into time.Time.

package dbutil

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/go-sql-driver/mysql"
)

// SQL statement suffix to be appended when creating tables.
const SqlCreateTableSuffix = "CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci"

// Description of the SQL configuration file format.
const SqlConfigFileDescription = `File must contain a JSON object of the following form:
   {
    "dataSourceName": "[username[:password]@][protocol[(address)]]/dbname", (the connection string required by go-sql-driver; database name must be specified, query parameters are not supported)
    "tlsDisable": "false|true", (defaults to false; if set to true, uses an unencrypted connection; otherwise, the following fields are mandatory)
    "tlsServerName": "serverName", (the domain name of the SQL server for TLS)
    "rootCertPath": "/path/server-ca.pem", (the root certificate of the SQL server for TLS)
    "clientCertPath": "/path/client-cert.pem", (the client certificate for TLS)
    "clientKeyPath": "/path/client-key.pem" (the client private key for TLS)
   }`

// SqlConfig holds the fields needed to connect to a SQL instance and to
// configure TLS encryption of the information sent over the wire.
type SqlConfig struct {
	wrapped *iSqlConfig
}

type iSqlConfig struct {
	// DataSourceName is the connection string as required by go-sql-driver:
	// "[username[:password]@][protocol[(address)]]/dbname";
	// database name must be specified, query parameters are not supported.
	DataSourceName string `json:"dataSourceName"`
	// TLSDisable, if set to true, uses an unencrypted connection;
	// otherwise, the following fields are mandatory.
	TLSDisable bool `json:"tlsDisable"`
	// TLSServerName is the domain name of the SQL server for TLS.
	TLSServerName string `json:"tlsServerName"`
	// RootCertPath is the root certificate of the SQL server for TLS.
	RootCertPath string `json:"rootCertPath"`
	// ClientCertPath is the client certificate for TLS.
	ClientCertPath string `json:"clientCertPath"`
	// ClientKeyPath is the client private key for TLS.
	ClientKeyPath string `json:"clientKeyPath"`
	// tlsConfigIdentifier is the identifier under which the TLS configuration
	// is registered with go-sql-driver. It is computed as a secure hash of the
	// configuration file path and contents.
	tlsConfigIdentifier string `json:"-"`
}

// Parses the SQL configuration file pointed to by sqlConfigFile (format
// described in SqlConfigFileDescription; also see links below) and registers
// the TLS configuration with go-mysql-driver.
// https://github.com/go-sql-driver/mysql/#dsn-data-source-name
// https://github.com/go-sql-driver/mysql/#tls
func ParseSqlConfigFromFile(sqlConfigFile string) (*SqlConfig, error) {
	configJSON, err := ioutil.ReadFile(sqlConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed reading SQL config file %q: %v", sqlConfigFile, err)
	}
	var config iSqlConfig
	if err = json.Unmarshal(configJSON, &config); err != nil {
		return nil, fmt.Errorf("failed parsing SQL config file %q: %v", sqlConfigFile, err)
	}
	if !config.TLSDisable {
		rawHash := sha256.Sum256(append([]byte(sqlConfigFile+":"), configJSON...))
		config.tlsConfigIdentifier = hex.EncodeToString(rawHash[:])
		if err = registerSqlTLSConfig(&config); err != nil {
			return nil, fmt.Errorf("failed registering TLS config from %q: %v", sqlConfigFile, err)
		}
	}
	return &SqlConfig{wrapped: &config}, nil
}

// Opens a connection to the SQL database using the provided configuration.
// Sets the specified transaction isolation (see link below).
// https://dev.mysql.com/doc/refman/5.5/en/server-system-variables.html#sysvar_tx_isolation
func NewSqlDBConn(sqlConfig *SqlConfig, txIsolation string) (*sql.DB, error) {
	return openSqlDBConn(configureSqlDBConn(sqlConfig.wrapped, txIsolation))
}

// Convenience function to parse the configuration file and open a connection
// to the SQL database. If multiple connections with the same configuration are
// needed, a single ParseSqlConfigFromFile() and multiple NewSqlDbConn() calls
// are recommended instead.
func NewSqlDBConnFromFile(sqlConfigFile, txIsolation string) (*sql.DB, error) {
	config, err := ParseSqlConfigFromFile(sqlConfigFile)
	if err != nil {
		return nil, err
	}
	return NewSqlDBConn(config, txIsolation)
}

func configureSqlDBConn(sqlConfig *iSqlConfig, txIsolation string) string {
	params := url.Values{}
	// Setting charset is unneccessary when collation is set, according to
	// https://github.com/go-sql-driver/mysql/#charset
	params.Set("collation", "utf8mb4_general_ci")
	// Maps SQL date/time values into time.Time instead of strings.
	params.Set("parseTime", "true")
	params.Set("loc", "UTC")
	params.Set("time_zone", "'+00:00'")
	if !sqlConfig.TLSDisable {
		params.Set("tls", sqlConfig.tlsConfigIdentifier)
	}
	params.Set("tx_isolation", "'"+txIsolation+"'")
	return sqlConfig.DataSourceName + "?" + params.Encode()
}

func openSqlDBConn(dataSrcName string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dataSrcName)
	if err != nil {
		return nil, fmt.Errorf("failed opening database connection at %q: %v", dataSrcName, err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed connecting to database at %q: %v", dataSrcName, err)
	}
	return db, nil
}

// registerSqlTLSConfig sets up the SQL connection to use TLS encryption.
// For more information see https://github.com/go-sql-driver/mysql/#tls
func registerSqlTLSConfig(config *iSqlConfig) error {
	rootCertPool := x509.NewCertPool()
	pem, err := ioutil.ReadFile(config.RootCertPath)
	if err != nil {
		return fmt.Errorf("failed reading root certificate: %v", err)
	}
	if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
		return fmt.Errorf("failed to append PEM to cert pool")
	}
	ckpair, err := tls.LoadX509KeyPair(config.ClientCertPath, config.ClientKeyPath)
	if err != nil {
		return fmt.Errorf("failed loading client key pair: %v", err)
	}
	clientCert := []tls.Certificate{ckpair}
	return mysql.RegisterTLSConfig(config.tlsConfigIdentifier, &tls.Config{
		RootCAs:      rootCertPool,
		Certificates: clientCert,
		ServerName:   config.TLSServerName,
		// SSLv3 is more vulnerable than TLSv1.0, see https://en.wikipedia.org/wiki/POODLE
		// TODO(ivanpi): Increase when Cloud SQL starts supporting higher TLS versions.
		MinVersion: tls.VersionTLS10,
		ClientAuth: tls.RequireAndVerifyClientCert,
	})
}
