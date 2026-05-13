// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package backup

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/mattn/go-sqlite3"
)

func backupOnline(ctx context.Context, srcDB *sql.DB, dstPath string) error {
	tmpPath := dstPath + ".tmp"

	dstDB, err := sql.Open("sqlite3", tmpPath)
	if err != nil {
		return fmt.Errorf("open destination: %w", err)
	}
	defer func() {
		dstDB.Close()
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	if err = dstDB.Ping(); err != nil {
		return fmt.Errorf("ping destination: %w", err)
	}

	srcConn, err := srcDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire source connection: %w", err)
	}
	defer srcConn.Close()

	dstConn, err := dstDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire destination connection: %w", err)
	}
	defer dstConn.Close()

	err = dstConn.Raw(func(dstRaw any) error {
		dstSQLiteConn, ok := dstRaw.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("destination is not a *sqlite3.SQLiteConn")
		}

		return srcConn.Raw(func(srcRaw any) error {
			srcSQLiteConn, ok := srcRaw.(*sqlite3.SQLiteConn)
			if !ok {
				return fmt.Errorf("source is not a *sqlite3.SQLiteConn")
			}

			bk, err := dstSQLiteConn.Backup("main", srcSQLiteConn, "main")
			if err != nil {
				return fmt.Errorf("init backup: %w", err)
			}

			done, err := bk.Step(-1)
			if err != nil {
				bk.Finish()
				return fmt.Errorf("backup step: %w", err)
			}
			if !done {
				bk.Finish()
				return fmt.Errorf("backup not completed")
			}

			return bk.Finish()
		})
	})

	if err != nil {
		return err
	}

	dstDB.Close()
	if err = os.Rename(tmpPath, dstPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename backup: %w", err)
	}

	return nil
}
