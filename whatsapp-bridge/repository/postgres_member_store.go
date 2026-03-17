package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"whatsapp-client/domain"
)

type PostgresMemberStore struct {
	db *sql.DB
}

func NewPostgresMemberStore(db *sql.DB) *PostgresMemberStore {
	return &PostgresMemberStore{db: db}
}

func (s *PostgresMemberStore) EnsureSchema(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS group_member_versions (
			seva_type TEXT NOT NULL,
			group_no  INTEGER NOT NULL,
			version   BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (seva_type, group_no)
		)`,
		`CREATE TABLE IF NOT EXISTS group_members (
			seva_type TEXT NOT NULL,
			group_no  INTEGER NOT NULL,
			name TEXT NOT NULL,
			adhyay_no INTEGER NOT NULL,
			phone_number TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (seva_type, group_no, name)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresMemberStore) GroupIsInitialized(sevaType domain.SevaType, groupNo int) (bool, error) {
	ctx := context.Background()

	var exists bool
	if err := s.db.QueryRowContext(
		ctx,
		`SELECT EXISTS(SELECT 1 FROM group_member_versions WHERE seva_type=$1 AND group_no=$2)`,
		string(sevaType),
		groupNo,
	).Scan(&exists); err != nil {
		return false, err
	}
	if exists {
		return true, nil
	}

	if err := s.db.QueryRowContext(
		ctx,
		`SELECT EXISTS(SELECT 1 FROM group_members WHERE seva_type=$1 AND group_no=$2)`,
		string(sevaType),
		groupNo,
	).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

func (s *PostgresMemberStore) GetGroupMembers(sevaType domain.SevaType, groupNo int) ([]domain.Member, int64, error) {
	ctx := context.Background()
	var version int64
	err := s.db.QueryRowContext(ctx, `SELECT version FROM group_member_versions WHERE seva_type=$1 AND group_no=$2`, string(sevaType), groupNo).Scan(&version)
	if err != nil {
		if err == sql.ErrNoRows {
			version = 0
		} else {
			return nil, 0, err
		}
	}

	rows, err := s.db.QueryContext(ctx, `SELECT name, adhyay_no, phone_number FROM group_members WHERE seva_type=$1 AND group_no=$2`, string(sevaType), groupNo)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	members := make([]domain.Member, 0)
	for rows.Next() {
		var name string
		var adhyayNo int
		var phone string
		if err := rows.Scan(&name, &adhyayNo, &phone); err != nil {
			return nil, 0, err
		}
		m := domain.Member{Name: name, AdhyayNo: adhyayNo}
		if phone != "" {
			m.PhoneNumber = phone
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	sort.Slice(members, func(i, j int) bool {
		if members[i].AdhyayNo == members[j].AdhyayNo {
			return members[i].Name < members[j].Name
		}
		return members[i].AdhyayNo < members[j].AdhyayNo
	})

	return members, version, nil
}

func (s *PostgresMemberStore) ReplaceGroupMembers(sevaType domain.SevaType, groupNo int, members []domain.Member, expectedVersion int64) (int64, error) {
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var currentVersion int64
	err = tx.QueryRowContext(ctx, `SELECT version FROM group_member_versions WHERE seva_type=$1 AND group_no=$2 FOR UPDATE`, string(sevaType), groupNo).Scan(&currentVersion)
	if err != nil {
		if err == sql.ErrNoRows {
			currentVersion = 0
			if _, err := tx.ExecContext(ctx, `INSERT INTO group_member_versions (seva_type, group_no, version) VALUES ($1,$2,0)`, string(sevaType), groupNo); err != nil {
				return 0, err
			}
		} else {
			return 0, err
		}
	}

	if expectedVersion != currentVersion {
		return 0, fmt.Errorf("conflict")
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE seva_type=$1 AND group_no=$2`, string(sevaType), groupNo); err != nil {
		return 0, err
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO group_members (seva_type, group_no, name, adhyay_no, phone_number) VALUES ($1,$2,$3,$4,$5)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	for _, m := range members {
		phone := m.PhoneNumber
		if phone == "" {
			phone = ""
		}
		if _, err := stmt.ExecContext(ctx, string(sevaType), groupNo, m.Name, m.AdhyayNo, phone); err != nil {
			return 0, err
		}
	}

	newVersion := currentVersion + 1
	if _, err := tx.ExecContext(ctx, `UPDATE group_member_versions SET version=$3 WHERE seva_type=$1 AND group_no=$2`, string(sevaType), groupNo, newVersion); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return newVersion, nil
}

func (s *PostgresMemberStore) ListAllGroupMembers() ([]GroupMemberRow, error) {
	ctx := context.Background()
	rows, err := s.db.QueryContext(ctx, `SELECT seva_type, group_no, name, adhyay_no, phone_number FROM group_members`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]GroupMemberRow, 0)
	for rows.Next() {
		var sevaType string
		var groupNo int
		var name string
		var adhyayNo int
		var phone string
		if err := rows.Scan(&sevaType, &groupNo, &name, &adhyayNo, &phone); err != nil {
			return nil, err
		}
		out = append(out, GroupMemberRow{SevaType: domain.SevaType(sevaType), GroupNo: groupNo, Name: name, AdhyayNo: adhyayNo, PhoneNumber: phone})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
