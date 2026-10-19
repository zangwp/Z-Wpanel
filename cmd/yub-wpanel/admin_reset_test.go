package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func resetFixture(t *testing.T) (*sql.DB, string, []byte) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "reset.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`PRAGMA foreign_keys=ON;
 CREATE TABLE admin_users(id INTEGER PRIMARY KEY, username TEXT, password_hash TEXT, updated_at TEXT);
 INSERT INTO admin_users VALUES(1,'old-user','old-web-hash','before');`); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"admin":{"username":"old-user","password_hash":"old-web-hash","extra":true},"basic_auth":{"username":"old-basic","password_hash":"old-basic-hash"},"panel":{"port":8443},"future_option":[1,2]}`)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return db, path, data
}

var resetTestCredentials = administratorResetCredentials{"wpadmin", "new-web-hash", "new-basic-hash"}

func assertOldResetAccount(t *testing.T, db *sql.DB) {
	t.Helper()
	var username, hash string
	if err := db.QueryRow("SELECT username,password_hash FROM admin_users WHERE id=1").Scan(&username, &hash); err != nil {
		t.Fatal(err)
	}
	if username != "old-user" || hash != "old-web-hash" {
		t.Fatal("failed reset changed the stored administrator")
	}
}

func TestAdministratorResetPreservesUnrelatedConfiguration(t *testing.T) {
	db, path, _ := resetFixture(t)
	if err := persistAdministratorReset(context.Background(), db, path, resetTestCredentials, writeAdministratorResetConfig); err != nil {
		t.Fatal(err)
	}
	var username, hash string
	if err := db.QueryRow("SELECT username,password_hash FROM admin_users WHERE id=1").Scan(&username, &hash); err != nil || username != "wpadmin" || hash != "new-web-hash" {
		t.Fatal("database identity was not reset")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	var future []int
	if err := json.Unmarshal(document["future_option"], &future); err != nil || len(future) != 2 || future[0] != 1 || future[1] != 2 {
		t.Fatal("unknown configuration lost")
	}
	var admin, basic struct {
		Username string `json:"username"`
		Password string `json:"password_hash"`
		Extra    bool   `json:"extra"`
	}
	_ = json.Unmarshal(document["admin"], &admin)
	_ = json.Unmarshal(document["basic_auth"], &basic)
	if admin.Username != "wpadmin" || admin.Password != "new-web-hash" || !admin.Extra || basic.Password != "new-basic-hash" {
		t.Fatal("credential sections were not correctly updated")
	}
}

func TestAdministratorResetRejectsMalformedConfigBeforeDatabaseWrites(t *testing.T) {
	for _, data := range []string{`{`, `null`, `{}`, `{"admin":{},"basic_auth":null}`} {
		t.Run(data, func(t *testing.T) {
			db, path, _ := resetFixture(t)
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			if err := persistAdministratorReset(context.Background(), db, path, resetTestCredentials, writeAdministratorResetConfig); err == nil {
				t.Fatal("bad config accepted")
			}
			assertOldResetAccount(t, db)
		})
	}
}

func TestAdministratorResetWriteFailureRollsBackDatabase(t *testing.T) {
	db, path, original := resetFixture(t)
	err := persistAdministratorReset(context.Background(), db, path, resetTestCredentials, func(string, []byte) error { return errors.New("injected write failure") })
	if err == nil {
		t.Fatal("config write failure hidden")
	}
	assertOldResetAccount(t, db)
	data, _ := os.ReadFile(path)
	if string(data) != string(original) {
		t.Fatal("failed write changed original config")
	}
}

func TestAdministratorResetDatabaseFailureDoesNotReplaceConfig(t *testing.T) {
	db, path, original := resetFixture(t)
	if _, err := db.Exec(`CREATE TRIGGER reject_reset BEFORE UPDATE ON admin_users BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if err := persistAdministratorReset(context.Background(), db, path, resetTestCredentials, writeAdministratorResetConfig); err == nil {
		t.Fatal("database failure hidden")
	}
	data, _ := os.ReadFile(path)
	if string(data) != string(original) {
		t.Fatal("database failure changed config")
	}
	assertOldResetAccount(t, db)
}

func TestAdministratorResetCommitFailureRestoresExactConfigAndDatabase(t *testing.T) {
	db, path, original := resetFixture(t)
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE parents(id INTEGER PRIMARY KEY);
 CREATE TABLE deferred_child(id INTEGER REFERENCES parents(id) DEFERRABLE INITIALLY DEFERRED);
 CREATE TRIGGER fail_commit AFTER UPDATE ON admin_users BEGIN INSERT INTO deferred_child VALUES(123); END;`); err != nil {
		t.Fatal(err)
	}
	if err := persistAdministratorReset(context.Background(), db, path, resetTestCredentials, writeAdministratorResetConfig); err == nil {
		t.Fatal("commit failure hidden")
	}
	data, _ := os.ReadFile(path)
	if string(data) != string(original) {
		t.Fatal("original config was not restored")
	}
	assertOldResetAccount(t, db)
}

func TestAdministratorResetCommitFailureExposesFailedCompensation(t *testing.T) {
	db, path, _ := resetFixture(t)
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE parents(id INTEGER PRIMARY KEY);
 CREATE TABLE deferred_child(id INTEGER REFERENCES parents(id) DEFERRABLE INITIALLY DEFERRED);
 CREATE TRIGGER fail_commit AFTER UPDATE ON admin_users BEGIN INSERT INTO deferred_child VALUES(123); END;`); err != nil {
		t.Fatal(err)
	}
	writes := 0
	err := persistAdministratorReset(context.Background(), db, path, resetTestCredentials, func(path string, data []byte) error {
		writes++
		if writes == 2 {
			return errors.New("injected restore failure")
		}
		return writeAdministratorResetConfig(path, data)
	})
	if err == nil || !strings.Contains(err.Error(), "需要人工核对") || writes != 2 {
		t.Fatalf("incomplete compensation not disclosed: %v", err)
	}
}

type resetFailingRandom struct{}

func (resetFailingRandom) Read([]byte) (int, error) { return 0, errors.New("injected entropy failure") }

func TestAdministratorResetEntropyFailureReturnsNoPassword(t *testing.T) {
	if password, err := randomResetPassword(resetFailingRandom{}); err == nil || password != "" {
		t.Fatal("random failure produced a credential")
	}
}
