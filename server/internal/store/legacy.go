package store

import (
	"fmt"
	"path"
	"strings"
	"time"
)

// LegacyDevicePrefixes were used before the unified account tree.
var LegacyDevicePrefixes = []string{"PC", "Android"}

// MigrateLegacyDevicePrefixes moves files from PC/ and Android/ into the account root.
func (s *Store) MigrateLegacyDevicePrefixes() (int, error) {
	rows, err := s.DB.Query(`SELECT id FROM users`)
	if err != nil {
		return 0, err
	}
	var userIDs []int64
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			_ = rows.Close()
			return 0, err
		}
		userIDs = append(userIDs, userID)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	total := 0
	for _, userID := range userIDs {
		for _, prefix := range LegacyDevicePrefixes {
			n, err := s.flattenLegacyPrefix(userID, prefix)
			if err != nil {
				return total, err
			}
			total += n
		}
	}
	return total, nil
}

func (s *Store) flattenLegacyPrefix(userID int64, prefix string) (int, error) {
	prefix = strings.Trim(prefix, "/")
	all, err := s.ListAllActive(userID)
	if err != nil {
		return 0, err
	}

	type move struct {
		oldPath string
		newPath string
	}
	var moves []move
	for _, f := range all {
		switch {
		case f.Path == prefix:
			if _, err := s.SoftDelete(userID, prefix); err == nil {
				moves = append(moves, move{oldPath: prefix, newPath: ""})
			}
		case strings.HasPrefix(f.Path, prefix+"/"):
			moves = append(moves, move{
				oldPath: f.Path,
				newPath: strings.TrimPrefix(f.Path, prefix+"/"),
			})
		}
	}
	if len(moves) == 0 {
		return 0, nil
	}

	moved := 0
	for _, m := range moves {
		if m.newPath == "" {
			moved++
			continue
		}
		if err := s.relocatePath(userID, m.oldPath, m.newPath); err != nil {
			return moved, fmt.Errorf("relocate %s -> %s: %w", m.oldPath, m.newPath, err)
		}
		moved++
	}
	return moved, nil
}

func (s *Store) relocatePath(userID int64, oldPath, newPath string) error {
	f, err := s.GetFileByPath(userID, oldPath)
	if err != nil {
		return err
	}
	if f.Deleted {
		return fmt.Errorf("path deleted: %s", oldPath)
	}
	if existing, err := s.GetFileByPath(userID, newPath); err == nil && existing != nil && !existing.Deleted {
		if existing.MTime.After(f.MTime) {
			_, err := s.SoftDelete(userID, oldPath)
			return err
		}
		if _, err := s.SoftDelete(userID, newPath); err != nil {
			return err
		}
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	ver, err := nextVersionTx(tx, userID)
	if err != nil {
		return err
	}
	res, err := tx.Exec(`
UPDATE files SET path=?, name=?, version=?, deleted=0
WHERE user_id=? AND path=?`,
		newPath, path.Base(newPath), ver, userID, oldPath,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("path not found: %s", oldPath)
	}
	if _, err := tx.Exec(
		`INSERT INTO changes(user_id, file_id, action, version, created_at) VALUES(?,?,?,?,?)`,
		userID, f.ID, "upsert", ver, time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return err
	}
	return tx.Commit()
}
