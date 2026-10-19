package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type administratorResetCredentials struct {
	username, webHash, basicHash string
}

func randomResetPassword(reader io.Reader) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	password := make([]byte, 24)
	for index := range password {
		value, err := rand.Int(reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", errors.New("安全随机数不可用，未重置账号")
		}
		password[index] = alphabet[value.Int64()]
	}
	return string(password), nil
}

func administratorResetConfig(data []byte, credentials administratorResetCredentials) ([]byte, error) {
	var document map[string]json.RawMessage
	if json.Unmarshal(data, &document) != nil || document == nil {
		return nil, errors.New("配置文件无效，未重置账号")
	}
	for name, hash := range map[string]string{"admin": credentials.webHash, "basic_auth": credentials.basicHash} {
		var section map[string]json.RawMessage
		if json.Unmarshal(document[name], &section) != nil || section == nil {
			return nil, fmt.Errorf("配置缺少有效的 %s，未重置账号", name)
		}
		section["username"], _ = json.Marshal(credentials.username)
		section["password_hash"], _ = json.Marshal(hash)
		document[name], _ = json.Marshal(section)
	}
	return json.MarshalIndent(document, "", "  ")
}

// Replace within the same directory so a failed write never truncates config.
func writeAdministratorResetConfig(path string, data []byte) (err error) {
	file, err := os.CreateTemp(filepath.Dir(path), ".account-reset-*")
	if err != nil {
		return err
	}
	defer func() { _ = file.Close(); _ = os.Remove(file.Name()) }()
	if err = file.Chmod(0600); err != nil {
		return err
	}
	if _, err = file.Write(data); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func persistAdministratorReset(ctx context.Context, db *sql.DB, path string, credentials administratorResetCredentials, write func(string, []byte) error) error {
	if db == nil || credentials.username == "" || credentials.webHash == "" || credentials.basicHash == "" {
		return errors.New("账号重置参数无效")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("读取配置身份失败: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("配置文件不是普通文件，未重置账号")
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置失败: %w", err)
	}
	updated, err := administratorResetConfig(original, credentials)
	if err != nil {
		return err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("获取账号事务连接失败: %w", err)
	}
	defer conn.Close()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开始账号事务失败: %w", err)
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin_users").Scan(&count); err != nil {
		return fmt.Errorf("读取管理员失败: %w", err)
	}
	var result sql.Result
	if count == 0 {
		result, err = tx.ExecContext(ctx, "INSERT INTO admin_users (id, username, password_hash) VALUES (1, ?, ?)", credentials.username, credentials.webHash)
	} else {
		result, err = tx.ExecContext(ctx, "UPDATE admin_users SET username=?, password_hash=?, updated_at=CURRENT_TIMESTAMP WHERE id=1", credentials.username, credentials.webHash)
	}
	if err != nil {
		return fmt.Errorf("更新管理员失败: %w", err)
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 1 {
		return errors.New("未找到唯一主管理员，未重置账号")
	}
	if err = write(path, updated); err != nil {
		return fmt.Errorf("保存账号配置失败，数据库未提交: %w", err)
	}
	if err = tx.Commit(); err != nil {
		// SQLite and a file cannot share a transaction. Restore the exact config
		// on a detected commit failure and expose any incomplete compensation.
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, rollbackErr := conn.ExecContext(rollbackCtx, "ROLLBACK")
		if rollbackErr != nil && strings.Contains(rollbackErr.Error(), "no transaction is active") {
			rollbackErr = nil
		}
		restoreErr := write(path, original)
		if restoreErr != nil || rollbackErr != nil {
			return errors.Join(fmt.Errorf("账号事务提交失败: %w", err), rollbackErr, restoreErr, errors.New("账号恢复未确认，需要人工核对"))
		}
		return fmt.Errorf("账号事务提交失败，已恢复原配置: %w", err)
	}
	return nil
}
