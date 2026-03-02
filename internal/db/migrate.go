package db

import "time"

func (s *Store) migrate() error {
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO migrations(version, applied_at) VALUES (1, ?)`,
		time.Now().UTC(),
	)
	return err
}
