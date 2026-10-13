package store

import (
	"database/sql"
	"encoding/json"
	"github.com/jinzhao1994/kist/internal/domain"
)

func scanSavedSearch(scan func(...any) error) (domain.SavedSearch, bool, error) {
	var s domain.SavedSearch
	var raw string
	err := scan(&s.ID, &s.Name, &raw)
	if err == sql.ErrNoRows {
		return s, false, nil
	}
	if err != nil {
		return s, false, err
	}
	err = json.Unmarshal([]byte(raw), &s.Definition)
	return s, err == nil, err
}
func SavedSearchByID(q Queryer, id domain.SavedSearchID) (domain.SavedSearch, bool, error) {
	return scanSavedSearch(q.QueryRow(`SELECT id,name,definition FROM saved_searches WHERE id=?`, id).Scan)
}
func SavedSearches(q Queryer) ([]domain.SavedSearch, error) {
	rows, err := q.Query(`SELECT id,name,definition FROM saved_searches ORDER BY name_fold,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.SavedSearch{}
	for rows.Next() {
		s, _, err := scanSavedSearch(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
func (v *ViewTx) GetSavedSearch(id domain.SavedSearchID) (domain.SavedSearch, bool, error) {
	return SavedSearchByID(v.tx, id)
}
func (v *ViewTx) InsertSavedSearch(s domain.SavedSearch) error {
	raw, err := json.Marshal(s.Definition)
	if err != nil {
		return err
	}
	_, err = v.tx.Exec(`INSERT INTO saved_searches(id,name,name_fold,definition) VALUES(?,?,?,?)`, s.ID, s.Name, domain.Fold(s.Name), string(raw))
	return err
}
func (v *ViewTx) UpdateSavedSearch(s domain.SavedSearch) error {
	raw, err := json.Marshal(s.Definition)
	if err != nil {
		return err
	}
	_, err = v.tx.Exec(`UPDATE saved_searches SET name=?,name_fold=?,definition=? WHERE id=?`, s.Name, domain.Fold(s.Name), string(raw), s.ID)
	return err
}
func (v *ViewTx) DeleteSavedSearch(id domain.SavedSearchID) error {
	_, err := v.tx.Exec(`DELETE FROM saved_searches WHERE id=?`, id)
	return err
}

func (v *ViewTx) SavedSearchByName(name string) (domain.SavedSearch, bool, error) {
	return scanSavedSearch(v.tx.QueryRow(`SELECT id,name,definition FROM saved_searches WHERE name_fold=?`, domain.Fold(name)).Scan)
}
