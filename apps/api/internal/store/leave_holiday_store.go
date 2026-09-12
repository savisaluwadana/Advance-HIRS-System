package store

import (
	"context"
	"time"

	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/model"
)

func (s *Store) ListHolidays(ctx context.Context, orgID string, year int) ([]model.CompanyHoliday, error) {
	if year == 0 {
		year = time.Now().UTC().Year()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, to_char(holiday_date,'YYYY-MM-DD'), name, COALESCE(location,''), created_at
		FROM company_holidays
		WHERE organization_id=$1::uuid AND extract(year from holiday_date)=$2
		ORDER BY holiday_date, name`, orgID, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var holidays []model.CompanyHoliday
	for rows.Next() {
		var holiday model.CompanyHoliday
		if err := rows.Scan(&holiday.ID, &holiday.Date, &holiday.Name, &holiday.Location, &holiday.CreatedAt); err != nil {
			return nil, err
		}
		holidays = append(holidays, holiday)
	}
	return holidays, rows.Err()
}

func (s *Store) CreateHoliday(ctx context.Context, orgID string, input model.CreateHoliday) (model.CompanyHoliday, error) {
	var holiday model.CompanyHoliday
	err := s.pool.QueryRow(ctx, `
		INSERT INTO company_holidays (organization_id, holiday_date, name, location)
		VALUES ($1::uuid, $2::date, $3, NULLIF($4,''))
		RETURNING id::text, to_char(holiday_date,'YYYY-MM-DD'), name, COALESCE(location,''), created_at`,
		orgID, input.Date, input.Name, input.Location).Scan(
		&holiday.ID, &holiday.Date, &holiday.Name, &holiday.Location, &holiday.CreatedAt,
	)
	return holiday, err
}

func (s *Store) DeleteHoliday(ctx context.Context, orgID, holidayID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM company_holidays WHERE organization_id=$1::uuid AND id=$2::uuid`, orgID, holidayID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
