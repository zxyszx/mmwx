package storage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	ErrParkingSpaceNotFound    = errors.New("parking space not found")
	ErrParkingMemberNotFound   = errors.New("parking member not found")
	ErrParkingSpaceFull        = errors.New("parking space is full")
	ErrParkingSlotOccupied     = errors.New("parking slot is occupied")
	ErrInvalidParkingSlot      = errors.New("parking slot is invalid")
	ErrParkingSpacePaused      = errors.New("parking space is paused")
	ErrParkingPlatformNotFound = errors.New("parking platform not found")
	ErrParkingPlatformInUse    = errors.New("parking platform is in use")
	ErrParkingPlatformExists   = errors.New("parking platform already exists")
	ErrParkingPlatformPaused   = errors.New("parking platform is paused")
	ErrInvalidParkingAmount    = errors.New("parking amount must be non-negative")
	ErrInvalidParkingMonths    = errors.New("parking renewal months must be between 1 and 120")
	ErrInvalidParkingOrder     = errors.New("parking space order is invalid")
	ErrInvalidPlatformDefaults = errors.New("parking platform defaults are invalid")
)

type ParkingPlatform struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Icon           string    `json:"icon"`
	Status         string    `json:"status"`
	Note           string    `json:"note"`
	DefaultSlots   int       `json:"default_slots"`
	PasswordLength int       `json:"password_length"`
	SortOrder      int       `json:"sort_order"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	SpaceCount     int       `json:"space_count,omitempty"`
	TotalSlots     int       `json:"total_slots,omitempty"`
}

type ParkingSpace struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Password     string    `json:"password,omitempty"`
	PasswordSet  bool      `json:"password_set"`
	PlatformID   int64     `json:"platform_id"`
	Platform     string    `json:"platform"`
	PlatformIcon string    `json:"platform_icon"`
	SlotNumber   int       `json:"slot_number"`
	BillingDay   int       `json:"billing_day"`
	CardLast4    string    `json:"card_last4"`
	Location     string    `json:"location"`
	Tags         []string  `json:"tags"`
	TotalSlots   int       `json:"total_slots"`
	MonthlyPrice float64   `json:"monthly_price"`
	Status       string    `json:"status"`
	Note         string    `json:"note"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	MemberCount  int       `json:"member_count,omitempty"`
	SortOrder    int       `json:"sort_order"`
}

type ParkingMember struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Contact      string    `json:"contact"`
	ContactType  string    `json:"contact_type"`
	Telegram     string    `json:"telegram"`
	CarPlate     string    `json:"car_plate"`
	SpaceID      int64     `json:"space_id"`
	SpaceName    string    `json:"space_name,omitempty"`
	SlotLabel    string    `json:"slot_label"`
	StartDate    time.Time `json:"start_date"`
	ExpireDate   time.Time `json:"expire_date"`
	Status       string    `json:"status"`
	Amount       float64   `json:"amount"`
	Payment      string    `json:"payment"`
	Note         string    `json:"note"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	RenewalCount int       `json:"renewal_count,omitempty"`
}

type ParkingRenewal struct {
	ID            int64     `json:"id"`
	MemberID      int64     `json:"member_id"`
	MemberName    string    `json:"member_name,omitempty"`
	SpaceID       int64     `json:"space_id"`
	SpaceName     string    `json:"space_name,omitempty"`
	OldExpireDate time.Time `json:"old_expire_date"`
	NewExpireDate time.Time `json:"new_expire_date"`
	Months        int       `json:"months"`
	Amount        float64   `json:"amount"`
	Payment       string    `json:"payment"`
	Operator      string    `json:"operator"`
	Note          string    `json:"note"`
	CreatedAt     time.Time `json:"created_at"`
}

type ParkingSubscription struct {
	ID            int64     `json:"id"`
	ServiceName   string    `json:"service_name"`
	AccountName   string    `json:"account_name"`
	StartDate     time.Time `json:"start_date"`
	ExpireDate    time.Time `json:"expire_date"`
	RenewalMonths int       `json:"renewal_months"`
	Amount        float64   `json:"amount"`
	ReminderDays  int       `json:"reminder_days"`
	Note          string    `json:"note"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ParkingSummary struct {
	Spaces         int     `json:"spaces"`
	TotalSlots     int     `json:"total_slots"`
	ActiveMembers  int     `json:"active_members"`
	AvailableSlots int     `json:"available_slots"`
	ExpiringToday  int     `json:"expiring_today"`
	Expiring7Days  int     `json:"expiring_7_days"`
	Expiring30Days int     `json:"expiring_30_days"`
	Expired        int     `json:"expired"`
	MonthlyRevenue float64 `json:"monthly_revenue"`
}

func (r *TrafficRepository) migrateParkingTables() error {
	const schema = `
	CREATE TABLE IF NOT EXISTS parking_platforms (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    name TEXT NOT NULL COLLATE NOCASE UNIQUE,
	    status TEXT NOT NULL DEFAULT 'active',
	    note TEXT NOT NULL DEFAULT '',
	    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_parking_platforms_status ON parking_platforms(status);

	CREATE TABLE IF NOT EXISTS parking_spaces (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    platform TEXT NOT NULL DEFAULT '',
    location TEXT NOT NULL DEFAULT '',
    tags_json TEXT NOT NULL DEFAULT '[]',
    total_slots INTEGER NOT NULL DEFAULT 1,
    monthly_price REAL NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_parking_spaces_status ON parking_spaces(status);

CREATE TABLE IF NOT EXISTS parking_members (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    contact TEXT NOT NULL DEFAULT '',
    telegram TEXT NOT NULL DEFAULT '',
    car_plate TEXT NOT NULL DEFAULT '',
    space_id INTEGER NOT NULL,
    slot_label TEXT NOT NULL DEFAULT '',
    start_date TIMESTAMP NOT NULL,
    expire_date TIMESTAMP NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    amount REAL NOT NULL DEFAULT 0,
    payment TEXT NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(space_id) REFERENCES parking_spaces(id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_parking_members_space ON parking_members(space_id);
CREATE INDEX IF NOT EXISTS idx_parking_members_expire ON parking_members(expire_date);
CREATE INDEX IF NOT EXISTS idx_parking_members_status ON parking_members(status);

CREATE TABLE IF NOT EXISTS parking_renewals (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    member_id INTEGER NOT NULL,
    space_id INTEGER NOT NULL,
    old_expire_date TIMESTAMP NOT NULL,
    new_expire_date TIMESTAMP NOT NULL,
    months INTEGER NOT NULL DEFAULT 1,
    amount REAL NOT NULL DEFAULT 0,
    payment TEXT NOT NULL DEFAULT '',
    operator TEXT NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(member_id) REFERENCES parking_members(id) ON DELETE RESTRICT,
    FOREIGN KEY(space_id) REFERENCES parking_spaces(id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_parking_renewals_created ON parking_renewals(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_parking_renewals_member ON parking_renewals(member_id);

CREATE TABLE IF NOT EXISTS parking_subscriptions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    service_name TEXT NOT NULL,
    account_name TEXT NOT NULL DEFAULT '',
    start_date TIMESTAMP NOT NULL,
    expire_date TIMESTAMP NOT NULL,
    renewal_months INTEGER NOT NULL DEFAULT 1,
    amount REAL NOT NULL DEFAULT 0,
    reminder_days INTEGER NOT NULL DEFAULT 7,
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_parking_subscriptions_expire ON parking_subscriptions(expire_date ASC);
`
	if _, err := r.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate parking tables: %w", err)
	}
	for _, column := range []struct {
		name       string
		definition string
	}{
		{name: "default_slots", definition: "INTEGER NOT NULL DEFAULT 1"},
		{name: "password_length", definition: "INTEGER NOT NULL DEFAULT 8"},
		{name: "icon", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "sort_order", definition: "INTEGER NOT NULL DEFAULT 0"},
	} {
		if err := r.ensureParkingColumn("parking_platforms", column.name, column.definition); err != nil {
			return err
		}
	}
	if err := r.ensureParkingColumn("parking_members", "contact_type", "TEXT NOT NULL DEFAULT 'wechat'"); err != nil {
		return err
	}
	for _, column := range []struct {
		name       string
		definition string
	}{
		{name: "platform", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "platform_id", definition: "INTEGER NOT NULL DEFAULT 0"},
		{name: "tags_json", definition: "TEXT NOT NULL DEFAULT '[]'"},
		{name: "sort_order", definition: "INTEGER NOT NULL DEFAULT 0"},
		{name: "password_cipher", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "slot_number", definition: "INTEGER NOT NULL DEFAULT 0"},
		{name: "billing_day", definition: "INTEGER NOT NULL DEFAULT 1"},
		{name: "card_last4", definition: "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := r.ensureParkingSpaceColumn(column.name, column.definition); err != nil {
			return err
		}
	}
	if _, err := r.db.Exec(`
INSERT OR IGNORE INTO parking_platforms (name)
SELECT DISTINCT TRIM(platform) FROM parking_spaces WHERE TRIM(platform) != '';
UPDATE parking_spaces
SET platform_id = COALESCE((SELECT id FROM parking_platforms WHERE name = parking_spaces.platform COLLATE NOCASE LIMIT 1), 0)
WHERE platform_id = 0 AND TRIM(platform) != '';
UPDATE parking_platforms SET sort_order = id WHERE sort_order = 0;
UPDATE parking_spaces SET sort_order = id WHERE sort_order = 0;
UPDATE parking_spaces SET slot_number = id WHERE slot_number = 0;`); err != nil {
		return fmt.Errorf("backfill parking platforms: %w", err)
	}
	return nil
}

func (r *TrafficRepository) encryptParkingPassword(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	block, err := aes.NewCipher(r.parkingKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(value), nil)
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (r *TrafficRepository) decryptParkingPassword(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	data, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(r.parkingKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("invalid encrypted parking password")
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (r *TrafficRepository) ensureParkingSpaceColumn(name, definition string) error {
	return r.ensureParkingColumn("parking_spaces", name, definition)
}

func (r *TrafficRepository) ensureParkingColumn(table, name, definition string) error {
	rows, err := r.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("%s table info: %w", table, err)
	}
	found := false
	for rows.Next() {
		var (
			cid        int
			columnName string
			columnType string
			notNull    int
			defaultVal sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultVal, &primaryKey); err != nil {
			rows.Close()
			return fmt.Errorf("scan %s table info: %w", table, err)
		}
		if strings.EqualFold(columnName, name) {
			found = true
			break
		}
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close %s table info: %w", table, err)
	}
	if found {
		return nil
	}
	if _, err := r.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, name, definition)); err != nil {
		return fmt.Errorf("add %s column %s: %w", table, name, err)
	}
	return nil
}

func normalizeParkingTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	normalized := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		if len([]rune(tag)) > 24 {
			tag = string([]rune(tag)[:24])
		}
		key := strings.ToLower(tag)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, tag)
		if len(normalized) == 12 {
			break
		}
	}
	return normalized
}

func encodeParkingTags(tags []string) string {
	encoded, err := json.Marshal(normalizeParkingTags(tags))
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func decodeParkingTags(encoded string) []string {
	var tags []string
	if err := json.Unmarshal([]byte(encoded), &tags); err != nil {
		return []string{}
	}
	return normalizeParkingTags(tags)
}

func normalizeParkingStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "paused":
		return "paused"
	default:
		return "active"
	}
}

func normalizeParkingPlatformStatus(status string) string {
	return normalizeParkingStatus(status)
}

func normalizePlatformDefaults(item *ParkingPlatform) error {
	if item.DefaultSlots == 0 {
		item.DefaultSlots = 1
	}
	if item.PasswordLength == 0 {
		item.PasswordLength = 8
	}
	if item.DefaultSlots < 1 || item.DefaultSlots > 100 || item.PasswordLength < 1 || item.PasswordLength > 128 {
		return ErrInvalidPlatformDefaults
	}
	return nil
}

func normalizeMemberStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "exited":
		return "exited"
	default:
		return "active"
	}
}

func (r *TrafficRepository) ListParkingPlatforms(ctx context.Context) ([]ParkingPlatform, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT p.id, p.name, p.icon, p.status, p.note, p.default_slots, p.password_length, p.sort_order, p.created_at, p.updated_at,
       COUNT(s.id), COALESCE(SUM(s.total_slots), 0)
FROM parking_platforms p
LEFT JOIN parking_spaces s ON s.platform_id = p.id
GROUP BY p.id
ORDER BY p.sort_order ASC, p.id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ParkingPlatform
	for rows.Next() {
		var item ParkingPlatform
		if err := rows.Scan(&item.ID, &item.Name, &item.Icon, &item.Status, &item.Note, &item.DefaultSlots, &item.PasswordLength, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt, &item.SpaceCount, &item.TotalSlots); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *TrafficRepository) GetParkingPlatform(ctx context.Context, id int64) (ParkingPlatform, error) {
	var item ParkingPlatform
	err := r.db.QueryRowContext(ctx, `
SELECT p.id, p.name, p.icon, p.status, p.note, p.default_slots, p.password_length, p.sort_order, p.created_at, p.updated_at,
       COUNT(s.id), COALESCE(SUM(s.total_slots), 0)
FROM parking_platforms p
LEFT JOIN parking_spaces s ON s.platform_id = p.id
WHERE p.id = ?
GROUP BY p.id`, id).Scan(&item.ID, &item.Name, &item.Icon, &item.Status, &item.Note, &item.DefaultSlots, &item.PasswordLength, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt, &item.SpaceCount, &item.TotalSlots)
	if errors.Is(err, sql.ErrNoRows) {
		return ParkingPlatform{}, ErrParkingPlatformNotFound
	}
	return item, err
}

func (r *TrafficRepository) CreateParkingPlatform(ctx context.Context, item ParkingPlatform) (ParkingPlatform, error) {
	item.Name = strings.TrimSpace(item.Name)
	item.Status = normalizeParkingPlatformStatus(item.Status)
	if err := normalizePlatformDefaults(&item); err != nil {
		return ParkingPlatform{}, err
	}
	_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM parking_platforms`).Scan(&item.SortOrder)
	res, err := r.db.ExecContext(ctx, `INSERT INTO parking_platforms (name, icon, status, note, default_slots, password_length, sort_order) VALUES (?, ?, ?, ?, ?, ?, ?)`, item.Name, strings.TrimSpace(item.Icon), item.Status, strings.TrimSpace(item.Note), item.DefaultSlots, item.PasswordLength, item.SortOrder)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return ParkingPlatform{}, ErrParkingPlatformExists
		}
		return ParkingPlatform{}, err
	}
	id, _ := res.LastInsertId()
	return r.GetParkingPlatform(ctx, id)
}

func (r *TrafficRepository) UpdateParkingPlatform(ctx context.Context, item ParkingPlatform) (ParkingPlatform, error) {
	item.Name = strings.TrimSpace(item.Name)
	item.Status = normalizeParkingPlatformStatus(item.Status)
	if err := normalizePlatformDefaults(&item); err != nil {
		return ParkingPlatform{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ParkingPlatform{}, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE parking_platforms SET name = ?, icon = ?, status = ?, note = ?, default_slots = ?, password_length = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, item.Name, strings.TrimSpace(item.Icon), item.Status, strings.TrimSpace(item.Note), item.DefaultSlots, item.PasswordLength, item.ID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return ParkingPlatform{}, ErrParkingPlatformExists
		}
		return ParkingPlatform{}, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ParkingPlatform{}, ErrParkingPlatformNotFound
	}
	if _, err := tx.ExecContext(ctx, `UPDATE parking_spaces SET platform = ?, updated_at = CURRENT_TIMESTAMP WHERE platform_id = ?`, item.Name, item.ID); err != nil {
		return ParkingPlatform{}, err
	}
	if err := tx.Commit(); err != nil {
		return ParkingPlatform{}, err
	}
	return r.GetParkingPlatform(ctx, item.ID)
}

func (r *TrafficRepository) DeleteParkingPlatform(ctx context.Context, id int64) error {
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM parking_spaces WHERE platform_id = ?`, id).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return ErrParkingPlatformInUse
	}
	res, err := r.db.ExecContext(ctx, `DELETE FROM parking_platforms WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrParkingPlatformNotFound
	}
	return nil
}

func (r *TrafficRepository) ReorderParkingPlatforms(ctx context.Context, ids []int64) error {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM parking_platforms`).Scan(&total); err != nil {
		return err
	}
	if len(ids) != total {
		return ErrInvalidParkingOrder
	}
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return ErrInvalidParkingOrder
		}
		if _, exists := seen[id]; exists {
			return ErrInvalidParkingOrder
		}
		seen[id] = struct{}{}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for index, id := range ids {
		res, err := tx.ExecContext(ctx, `UPDATE parking_platforms SET sort_order = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, index+1, id)
		if err != nil {
			return err
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return ErrParkingPlatformNotFound
		}
	}
	return tx.Commit()
}

func (r *TrafficRepository) ParkingSummary(ctx context.Context) (ParkingSummary, error) {
	var out ParkingSummary
	row := r.db.QueryRowContext(ctx, `
SELECT
  COALESCE(COUNT(*), 0),
  COALESCE(SUM(total_slots), 0)
FROM parking_spaces
WHERE status = 'active'`)
	if err := row.Scan(&out.Spaces, &out.TotalSlots); err != nil {
		return out, err
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	in7 := today.AddDate(0, 0, 7)
	in30 := today.AddDate(0, 0, 30)
	row = r.db.QueryRowContext(ctx, `
SELECT
  COALESCE(SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END), 0),
  COALESCE(SUM(CASE WHEN status = 'active' AND date(expire_date) = date(?) THEN 1 ELSE 0 END), 0),
  COALESCE(SUM(CASE WHEN status = 'active' AND expire_date > ? AND expire_date <= ? THEN 1 ELSE 0 END), 0),
  COALESCE(SUM(CASE WHEN status = 'active' AND expire_date > ? AND expire_date <= ? THEN 1 ELSE 0 END), 0),
  COALESCE(SUM(CASE WHEN status = 'active' AND expire_date < ? THEN 1 ELSE 0 END), 0)
FROM parking_members`, today, today, in7, today, in30, today)
	if err := row.Scan(&out.ActiveMembers, &out.ExpiringToday, &out.Expiring7Days, &out.Expiring30Days, &out.Expired); err != nil {
		return out, err
	}
	out.AvailableSlots = out.TotalSlots - out.ActiveMembers
	if out.AvailableSlots < 0 {
		out.AvailableSlots = 0
	}

	monthStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
	monthEnd := monthStart.AddDate(0, 1, 0)
	_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount), 0) FROM parking_renewals WHERE created_at >= ? AND created_at < ?`, monthStart, monthEnd).Scan(&out.MonthlyRevenue)
	return out, nil
}

func (r *TrafficRepository) ListParkingSpaces(ctx context.Context) ([]ParkingSpace, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT s.id, s.name, s.password_cipher != '', s.platform_id, COALESCE(NULLIF(p.name, ''), s.platform), COALESCE(p.icon, ''), s.slot_number, s.billing_day, s.card_last4, s.location, s.tags_json, s.total_slots, s.monthly_price, s.status, s.note, s.created_at, s.updated_at,
       s.sort_order,
       COALESCE(SUM(CASE WHEN m.status = 'active' THEN 1 ELSE 0 END), 0) AS member_count
FROM parking_spaces s
LEFT JOIN parking_platforms p ON p.id = s.platform_id
LEFT JOIN parking_members m ON m.space_id = s.id
GROUP BY s.id
ORDER BY s.sort_order ASC, s.id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ParkingSpace
	for rows.Next() {
		var item ParkingSpace
		var tagsJSON string
		if err := rows.Scan(&item.ID, &item.Name, &item.PasswordSet, &item.PlatformID, &item.Platform, &item.PlatformIcon, &item.SlotNumber, &item.BillingDay, &item.CardLast4, &item.Location, &tagsJSON, &item.TotalSlots, &item.MonthlyPrice, &item.Status, &item.Note, &item.CreatedAt, &item.UpdatedAt, &item.SortOrder, &item.MemberCount); err != nil {
			return nil, err
		}
		item.Tags = decodeParkingTags(tagsJSON)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *TrafficRepository) CreateParkingSpace(ctx context.Context, item ParkingSpace) (ParkingSpace, error) {
	if item.MonthlyPrice < 0 {
		return ParkingSpace{}, ErrInvalidParkingAmount
	}
	if item.TotalSlots <= 0 {
		item.TotalSlots = 1
	}
	if item.BillingDay < 1 || item.BillingDay > 31 {
		item.BillingDay = 1
	}
	item.CardLast4 = strings.TrimSpace(item.CardLast4)
	if len(item.CardLast4) > 4 {
		item.CardLast4 = item.CardLast4[len(item.CardLast4)-4:]
	}
	item.Status = normalizeParkingStatus(item.Status)
	platform, err := r.GetParkingPlatform(ctx, item.PlatformID)
	if err != nil {
		return ParkingSpace{}, err
	}
	if platform.Status != "active" {
		return ParkingSpace{}, ErrParkingPlatformPaused
	}
	var sortOrder int
	_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM parking_spaces`).Scan(&sortOrder)
	var slotNumber int
	_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(slot_number), 0) + 1 FROM parking_spaces WHERE platform_id = ?`, platform.ID).Scan(&slotNumber)
	passwordCipher, err := r.encryptParkingPassword(item.Password)
	if err != nil {
		return ParkingSpace{}, err
	}
	res, err := r.db.ExecContext(ctx, `INSERT INTO parking_spaces (name, password_cipher, platform_id, platform, slot_number, billing_day, card_last4, location, tags_json, total_slots, monthly_price, status, note, sort_order) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, strings.TrimSpace(item.Name), passwordCipher, platform.ID, platform.Name, slotNumber, item.BillingDay, item.CardLast4, strings.TrimSpace(item.Location), encodeParkingTags(item.Tags), item.TotalSlots, item.MonthlyPrice, item.Status, strings.TrimSpace(item.Note), sortOrder)
	if err != nil {
		return ParkingSpace{}, err
	}
	id, _ := res.LastInsertId()
	return r.GetParkingSpace(ctx, id)
}

func (r *TrafficRepository) UpdateParkingSpace(ctx context.Context, item ParkingSpace) (ParkingSpace, error) {
	if item.ID <= 0 {
		return ParkingSpace{}, ErrParkingSpaceNotFound
	}
	if item.MonthlyPrice < 0 {
		return ParkingSpace{}, ErrInvalidParkingAmount
	}
	if item.TotalSlots <= 0 {
		item.TotalSlots = 1
	}
	existing, err := r.GetParkingSpace(ctx, item.ID)
	if err != nil {
		return ParkingSpace{}, err
	}
	if item.BillingDay < 1 || item.BillingDay > 31 {
		item.BillingDay = existing.BillingDay
	}
	item.CardLast4 = strings.TrimSpace(item.CardLast4)
	if len(item.CardLast4) > 4 {
		item.CardLast4 = item.CardLast4[len(item.CardLast4)-4:]
	}
	activeMembers, err := r.countActiveParkingMembers(ctx, item.ID, 0)
	if err != nil {
		return ParkingSpace{}, err
	}
	if activeMembers > item.TotalSlots {
		return ParkingSpace{}, ErrParkingSpaceFull
	}
	item.Status = normalizeParkingStatus(item.Status)
	platform, err := r.GetParkingPlatform(ctx, item.PlatformID)
	if err != nil {
		return ParkingSpace{}, err
	}
	if platform.Status != "active" && existing.PlatformID != platform.ID {
		return ParkingSpace{}, ErrParkingPlatformPaused
	}
	passwordCipher := ""
	if strings.TrimSpace(item.Password) != "" {
		passwordCipher, err = r.encryptParkingPassword(item.Password)
		if err != nil {
			return ParkingSpace{}, err
		}
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE parking_spaces
SET name = ?, password_cipher = CASE WHEN ? = '' THEN password_cipher ELSE ? END, platform_id = ?, platform = ?, billing_day = ?, card_last4 = ?, location = ?, tags_json = ?, total_slots = ?, monthly_price = ?, status = ?, note = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`, strings.TrimSpace(item.Name), passwordCipher, passwordCipher, platform.ID, platform.Name, item.BillingDay, item.CardLast4, strings.TrimSpace(item.Location), encodeParkingTags(item.Tags), item.TotalSlots, item.MonthlyPrice, item.Status, strings.TrimSpace(item.Note), item.ID)
	if err != nil {
		return ParkingSpace{}, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ParkingSpace{}, ErrParkingSpaceNotFound
	}
	return r.GetParkingSpace(ctx, item.ID)
}

func (r *TrafficRepository) GetParkingSpace(ctx context.Context, id int64) (ParkingSpace, error) {
	var item ParkingSpace
	var tagsJSON string
	err := r.db.QueryRowContext(ctx, `
SELECT s.id, s.name, s.platform, COALESCE(p.icon, ''), s.location, s.tags_json, s.total_slots, s.monthly_price, s.status, s.note, s.created_at, s.updated_at,
       s.platform_id, s.sort_order, s.password_cipher != '', s.slot_number, s.billing_day, s.card_last4, COALESCE(SUM(CASE WHEN m.status = 'active' THEN 1 ELSE 0 END), 0) AS member_count
FROM parking_spaces s
LEFT JOIN parking_platforms p ON p.id = s.platform_id
LEFT JOIN parking_members m ON m.space_id = s.id
WHERE s.id = ?
GROUP BY s.id`, id).Scan(&item.ID, &item.Name, &item.Platform, &item.PlatformIcon, &item.Location, &tagsJSON, &item.TotalSlots, &item.MonthlyPrice, &item.Status, &item.Note, &item.CreatedAt, &item.UpdatedAt, &item.PlatformID, &item.SortOrder, &item.PasswordSet, &item.SlotNumber, &item.BillingDay, &item.CardLast4, &item.MemberCount)
	if errors.Is(err, sql.ErrNoRows) {
		return ParkingSpace{}, ErrParkingSpaceNotFound
	}
	item.Tags = decodeParkingTags(tagsJSON)
	return item, err
}

func (r *TrafficRepository) GetParkingSpacePassword(ctx context.Context, id int64) (string, error) {
	var encrypted string
	err := r.db.QueryRowContext(ctx, `SELECT password_cipher FROM parking_spaces WHERE id = ?`, id).Scan(&encrypted)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrParkingSpaceNotFound
	}
	if err != nil {
		return "", err
	}
	return r.decryptParkingPassword(encrypted)
}

func (r *TrafficRepository) ReorderParkingSpaces(ctx context.Context, ids []int64) error {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM parking_spaces`).Scan(&total); err != nil {
		return err
	}
	if len(ids) != total {
		return ErrInvalidParkingOrder
	}
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return ErrInvalidParkingOrder
		}
		if _, exists := seen[id]; exists {
			return ErrInvalidParkingOrder
		}
		seen[id] = struct{}{}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for index, id := range ids {
		res, err := tx.ExecContext(ctx, `UPDATE parking_spaces SET sort_order = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, index+1, id)
		if err != nil {
			return err
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return ErrParkingSpaceNotFound
		}
	}
	return tx.Commit()
}

func (r *TrafficRepository) ListParkingMembers(ctx context.Context) ([]ParkingMember, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT m.id, m.name, m.contact, m.contact_type, m.telegram, m.car_plate, m.space_id, s.name, m.slot_label, m.start_date, m.expire_date,
       m.status, m.amount, m.payment, m.note, m.created_at, m.updated_at,
       COALESCE(COUNT(pr.id), 0) AS renewal_count
FROM parking_members m
JOIN parking_spaces s ON s.id = m.space_id
LEFT JOIN parking_renewals pr ON pr.member_id = m.id
GROUP BY m.id
ORDER BY m.expire_date ASC, m.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ParkingMember
	for rows.Next() {
		var item ParkingMember
		if err := rows.Scan(&item.ID, &item.Name, &item.Contact, &item.ContactType, &item.Telegram, &item.CarPlate, &item.SpaceID, &item.SpaceName, &item.SlotLabel, &item.StartDate, &item.ExpireDate, &item.Status, &item.Amount, &item.Payment, &item.Note, &item.CreatedAt, &item.UpdatedAt, &item.RenewalCount); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *TrafficRepository) CreateParkingMember(ctx context.Context, item ParkingMember) (ParkingMember, error) {
	if item.Amount < 0 {
		return ParkingMember{}, ErrInvalidParkingAmount
	}
	item.Status = normalizeMemberStatus(item.Status)
	if item.Status == "active" {
		if err := r.ensureParkingSpaceAvailable(ctx, item.SpaceID, 0); err != nil {
			return ParkingMember{}, err
		}
		label, err := r.assignParkingSlot(ctx, item.SpaceID, 0, item.SlotLabel)
		if err != nil {
			return ParkingMember{}, err
		}
		item.SlotLabel = label
	}
	if item.StartDate.IsZero() {
		item.StartDate = time.Now()
	}
	if item.ExpireDate.IsZero() {
		item.ExpireDate = item.StartDate.AddDate(0, 1, 0)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ParkingMember{}, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `
INSERT INTO parking_members (name, contact, contact_type, telegram, car_plate, space_id, slot_label, start_date, expire_date, status, amount, payment, note)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, strings.TrimSpace(item.Name), strings.TrimSpace(item.Contact), strings.TrimSpace(item.ContactType), strings.TrimSpace(item.Telegram), strings.TrimSpace(item.CarPlate), item.SpaceID, strings.TrimSpace(item.SlotLabel), item.StartDate, item.ExpireDate, item.Status, item.Amount, strings.TrimSpace(item.Payment), strings.TrimSpace(item.Note))
	if err != nil {
		return ParkingMember{}, err
	}
	id, _ := res.LastInsertId()
	months := (item.ExpireDate.Year()-item.StartDate.Year())*12 + int(item.ExpireDate.Month()-item.StartDate.Month())
	if months < 1 {
		months = 1
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO parking_renewals (member_id, space_id, old_expire_date, new_expire_date, months, amount, payment, operator, note, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, '', ?, ?)`, id, item.SpaceID, item.StartDate, item.ExpireDate, months, item.Amount, strings.TrimSpace(item.Payment), strings.TrimSpace(item.Note), item.StartDate); err != nil {
		return ParkingMember{}, err
	}
	if err := tx.Commit(); err != nil {
		return ParkingMember{}, err
	}
	return r.GetParkingMember(ctx, id)
}

func (r *TrafficRepository) UpdateParkingMember(ctx context.Context, item ParkingMember) (ParkingMember, error) {
	if item.ID <= 0 {
		return ParkingMember{}, ErrParkingMemberNotFound
	}
	if item.Amount < 0 {
		return ParkingMember{}, ErrInvalidParkingAmount
	}
	item.Status = normalizeMemberStatus(item.Status)
	if item.Status == "active" {
		if err := r.ensureParkingSpaceAvailable(ctx, item.SpaceID, item.ID); err != nil {
			return ParkingMember{}, err
		}
		label, err := r.assignParkingSlot(ctx, item.SpaceID, item.ID, item.SlotLabel)
		if err != nil {
			return ParkingMember{}, err
		}
		item.SlotLabel = label
	}
	if item.StartDate.IsZero() {
		item.StartDate = time.Now()
	}
	if item.ExpireDate.IsZero() {
		item.ExpireDate = item.StartDate.AddDate(0, 1, 0)
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE parking_members
SET name = ?, contact = ?, contact_type = ?, telegram = ?, car_plate = ?, space_id = ?, slot_label = ?, start_date = ?, expire_date = ?, status = ?, amount = ?, payment = ?, note = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`, strings.TrimSpace(item.Name), strings.TrimSpace(item.Contact), strings.TrimSpace(item.ContactType), strings.TrimSpace(item.Telegram), strings.TrimSpace(item.CarPlate), item.SpaceID, strings.TrimSpace(item.SlotLabel), item.StartDate, item.ExpireDate, item.Status, item.Amount, strings.TrimSpace(item.Payment), strings.TrimSpace(item.Note), item.ID)
	if err != nil {
		return ParkingMember{}, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ParkingMember{}, ErrParkingMemberNotFound
	}
	return r.GetParkingMember(ctx, item.ID)
}

func (r *TrafficRepository) ExitParkingMember(ctx context.Context, memberID int64) (ParkingMember, error) {
	res, err := r.db.ExecContext(ctx, `UPDATE parking_members SET status = 'exited', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, memberID)
	if err != nil {
		return ParkingMember{}, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ParkingMember{}, ErrParkingMemberNotFound
	}
	return r.GetParkingMember(ctx, memberID)
}

func (r *TrafficRepository) ensureParkingSpaceAvailable(ctx context.Context, spaceID, excludeMemberID int64) error {
	space, err := r.GetParkingSpace(ctx, spaceID)
	if err != nil {
		return err
	}
	if space.Status != "active" {
		return ErrParkingSpacePaused
	}
	activeMembers, err := r.countActiveParkingMembers(ctx, spaceID, excludeMemberID)
	if err != nil {
		return err
	}
	if activeMembers >= space.TotalSlots {
		return ErrParkingSpaceFull
	}
	return nil
}

func parkingSlotNumber(label string) int {
	label = strings.TrimSpace(label)
	if !strings.HasSuffix(label, "号位") {
		return 0
	}
	number, err := strconv.Atoi(strings.TrimSuffix(label, "号位"))
	if err != nil || number <= 0 {
		return 0
	}
	return number
}

func (r *TrafficRepository) assignParkingSlot(ctx context.Context, spaceID, excludeMemberID int64, label string) (string, error) {
	space, err := r.GetParkingSpace(ctx, spaceID)
	if err != nil {
		return "", err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT slot_label FROM parking_members WHERE space_id = ? AND status = 'active' AND id != ? ORDER BY id`, spaceID, excludeMemberID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	occupied := make(map[int]bool)
	var unnumbered int
	for rows.Next() {
		var existing string
		if err := rows.Scan(&existing); err != nil {
			return "", err
		}
		if number := parkingSlotNumber(existing); number > 0 && number <= space.TotalSlots {
			occupied[number] = true
		} else {
			unnumbered++
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	label = strings.TrimSpace(label)
	requested := parkingSlotNumber(label)
	if requested > space.TotalSlots {
		return "", ErrInvalidParkingSlot
	}
	if requested > 0 && occupied[requested] {
		return "", ErrParkingSlotOccupied
	}
	if requested > 0 {
		return label, nil
	}
	for slot := 1; slot <= space.TotalSlots; slot++ {
		if occupied[slot] {
			continue
		}
		if unnumbered > 0 {
			unnumbered--
			continue
		}
		if label == "" {
			return fmt.Sprintf("%d号位", slot), nil
		}
		return label, nil
	}
	return "", ErrParkingSpaceFull
}

func (r *TrafficRepository) countActiveParkingMembers(ctx context.Context, spaceID, excludeMemberID int64) (int, error) {
	query := `SELECT COALESCE(COUNT(*), 0) FROM parking_members WHERE space_id = ? AND status = 'active'`
	args := []any{spaceID}
	if excludeMemberID > 0 {
		query += ` AND id != ?`
		args = append(args, excludeMemberID)
	}
	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (r *TrafficRepository) GetParkingMember(ctx context.Context, id int64) (ParkingMember, error) {
	var item ParkingMember
	err := r.db.QueryRowContext(ctx, `
SELECT m.id, m.name, m.contact, m.contact_type, m.telegram, m.car_plate, m.space_id, s.name, m.slot_label, m.start_date, m.expire_date,
       m.status, m.amount, m.payment, m.note, m.created_at, m.updated_at,
       COALESCE(COUNT(pr.id), 0) AS renewal_count
FROM parking_members m
JOIN parking_spaces s ON s.id = m.space_id
LEFT JOIN parking_renewals pr ON pr.member_id = m.id
WHERE m.id = ?
GROUP BY m.id`, id).Scan(&item.ID, &item.Name, &item.Contact, &item.ContactType, &item.Telegram, &item.CarPlate, &item.SpaceID, &item.SpaceName, &item.SlotLabel, &item.StartDate, &item.ExpireDate, &item.Status, &item.Amount, &item.Payment, &item.Note, &item.CreatedAt, &item.UpdatedAt, &item.RenewalCount)
	if errors.Is(err, sql.ErrNoRows) {
		return ParkingMember{}, ErrParkingMemberNotFound
	}
	return item, err
}

func (r *TrafficRepository) RenewParkingMember(ctx context.Context, memberID int64, months int, amount float64, payment, operator, note string) (ParkingRenewal, error) {
	return r.RenewParkingMemberTo(ctx, memberID, months, amount, payment, operator, note, time.Time{})
}

func (r *TrafficRepository) RenewParkingMemberTo(ctx context.Context, memberID int64, months int, amount float64, payment, operator, note string, requestedExpire time.Time) (ParkingRenewal, error) {
	if requestedExpire.IsZero() && (months < 1 || months > 120) {
		return ParkingRenewal{}, ErrInvalidParkingMonths
	}
	if amount < 0 {
		return ParkingRenewal{}, ErrInvalidParkingAmount
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ParkingRenewal{}, err
	}
	defer tx.Rollback()

	var member ParkingMember
	err = tx.QueryRowContext(ctx, `SELECT id, space_id, expire_date FROM parking_members WHERE id = ?`, memberID).Scan(&member.ID, &member.SpaceID, &member.ExpireDate)
	if errors.Is(err, sql.ErrNoRows) {
		return ParkingRenewal{}, ErrParkingMemberNotFound
	}
	if err != nil {
		return ParkingRenewal{}, err
	}
	oldExpire := member.ExpireDate
	base := oldExpire
	if now := time.Now(); base.Before(now) {
		base = now
	}
	newExpire := base.AddDate(0, months, 0)
	if !requestedExpire.IsZero() {
		newExpire = requestedExpire
		if !newExpire.After(oldExpire) {
			return ParkingRenewal{}, ErrInvalidParkingMonths
		}
		if months < 1 {
			months = (newExpire.Year()-oldExpire.Year())*12 + int(newExpire.Month()-oldExpire.Month())
			if months < 1 {
				months = 1
			}
		}
	}
	res, err := tx.ExecContext(ctx, `
INSERT INTO parking_renewals (member_id, space_id, old_expire_date, new_expire_date, months, amount, payment, operator, note)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, memberID, member.SpaceID, oldExpire, newExpire, months, amount, strings.TrimSpace(payment), strings.TrimSpace(operator), strings.TrimSpace(note))
	if err != nil {
		return ParkingRenewal{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE parking_members SET expire_date = ?, amount = ?, payment = ?, status = 'active', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, newExpire, amount, strings.TrimSpace(payment), memberID); err != nil {
		return ParkingRenewal{}, err
	}
	if err := tx.Commit(); err != nil {
		return ParkingRenewal{}, err
	}
	id, _ := res.LastInsertId()
	return ParkingRenewal{ID: id, MemberID: memberID, SpaceID: member.SpaceID, OldExpireDate: oldExpire, NewExpireDate: newExpire, Months: months, Amount: amount, Payment: strings.TrimSpace(payment), Operator: strings.TrimSpace(operator), Note: strings.TrimSpace(note), CreatedAt: time.Now()}, nil
}

func (r *TrafficRepository) ListParkingRenewals(ctx context.Context) ([]ParkingRenewal, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT pr.id, pr.member_id, m.name, pr.space_id, s.name, pr.old_expire_date, pr.new_expire_date, pr.months, pr.amount, pr.payment, pr.operator, pr.note, pr.created_at
FROM parking_renewals pr
JOIN parking_members m ON m.id = pr.member_id
JOIN parking_spaces s ON s.id = pr.space_id
ORDER BY pr.created_at DESC, pr.id DESC
LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ParkingRenewal
	for rows.Next() {
		var item ParkingRenewal
		if err := rows.Scan(&item.ID, &item.MemberID, &item.MemberName, &item.SpaceID, &item.SpaceName, &item.OldExpireDate, &item.NewExpireDate, &item.Months, &item.Amount, &item.Payment, &item.Operator, &item.Note, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *TrafficRepository) ListParkingSubscriptions(ctx context.Context) ([]ParkingSubscription, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, service_name, account_name, start_date, expire_date, renewal_months, amount, reminder_days, note, created_at, updated_at
FROM parking_subscriptions
ORDER BY expire_date ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ParkingSubscription
	for rows.Next() {
		var item ParkingSubscription
		if err := rows.Scan(&item.ID, &item.ServiceName, &item.AccountName, &item.StartDate, &item.ExpireDate, &item.RenewalMonths, &item.Amount, &item.ReminderDays, &item.Note, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *TrafficRepository) CreateParkingSubscription(ctx context.Context, item ParkingSubscription) (ParkingSubscription, error) {
	result, err := r.db.ExecContext(ctx, `
INSERT INTO parking_subscriptions (service_name, account_name, start_date, expire_date, renewal_months, amount, reminder_days, note)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, strings.TrimSpace(item.ServiceName), strings.TrimSpace(item.AccountName), item.StartDate, item.ExpireDate, item.RenewalMonths, item.Amount, item.ReminderDays, strings.TrimSpace(item.Note))
	if err != nil {
		return ParkingSubscription{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return ParkingSubscription{}, err
	}
	item.ID = id
	item.ServiceName = strings.TrimSpace(item.ServiceName)
	item.AccountName = strings.TrimSpace(item.AccountName)
	item.Note = strings.TrimSpace(item.Note)
	item.CreatedAt = time.Now()
	item.UpdatedAt = item.CreatedAt
	return item, nil
}
