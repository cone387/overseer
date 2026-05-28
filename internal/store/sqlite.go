package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
	"golang.org/x/crypto/bcrypt"

	"github.com/overseer/overseer/internal/model"
	"github.com/overseer/overseer/internal/store/migrations"
)

// SQLiteStore implements the Store interface using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLiteStore with the given DSN.
// Use ":memory:" for in-memory databases or a file path for persistent storage.
func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Enable WAL mode and foreign keys for better performance and integrity.
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("exec pragma %q: %w", p, err)
		}
	}

	return &SQLiteStore{db: db}, nil
}

// allMigrations aggregates all migration SQL from the migrations package.
var allMigrations = append(append(append(append(migrations.InitialMigration, migrations.DevicesMigration...), migrations.ChannelsMigration...), migrations.AuthMigration...), migrations.SettingsMigration...)

// Migrate creates or upgrades the database schema.
func (s *SQLiteStore) Migrate() error {
	for _, stmt := range allMigrations {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}

	// Conditional migration: add 'type' column to devices if it doesn't exist.
	// SQLite doesn't support ALTER TABLE ADD COLUMN IF NOT EXISTS.
	if !s.columnExists("devices", "type") {
		if _, err := s.db.Exec(migrations.DesktopDevicesAlterSQL); err != nil {
			return fmt.Errorf("migrate (desktop devices alter): %w", err)
		}
	}

	// Run the remaining desktop devices migration statements (indexes etc.)
	for _, stmt := range migrations.DesktopDevicesMigration {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate (desktop devices): %w", err)
		}
	}

	return nil
}

// columnExists checks if a column exists in a table using pragma_table_info.
func (s *SQLiteStore) columnExists(table, column string) bool {
	query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?", table)
	var count int
	if err := s.db.QueryRow(query, column).Scan(&count); err != nil {
		return false
	}
	return count > 0
}

// Close closes the underlying database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// --- Message operations ---

func (s *SQLiteStore) SaveMessage(msg *model.Message) error {
	var extraJSON *string
	if len(msg.Extra) > 0 {
		b, err := json.Marshal(msg.Extra)
		if err != nil {
			return fmt.Errorf("marshal extra: %w", err)
		}
		str := string(b)
		extraJSON = &str
	}

	_, err := s.db.Exec(
		`INSERT INTO messages (id, source, channel, title, body, extra, status, fail_reason, retry_count, received_at, pushed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.Source, msg.Channel, msg.Title, msg.Body, extraJSON,
		string(msg.Status), msg.FailReason, msg.RetryCount, msg.ReceivedAt, msg.PushedAt,
	)
	if err != nil {
		return fmt.Errorf("save message: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdateMessageStatus(id string, status model.PushStatus, failReason string) error {
	var err error
	if status == model.StatusSuccess {
		now := time.Now()
		_, err = s.db.Exec(
			`UPDATE messages SET status = ?, fail_reason = ?, retry_count = retry_count + 1, pushed_at = ? WHERE id = ?`,
			string(status), failReason, now, id,
		)
	} else {
		_, err = s.db.Exec(
			`UPDATE messages SET status = ?, fail_reason = ?, retry_count = retry_count + 1 WHERE id = ?`,
			string(status), failReason, id,
		)
	}
	if err != nil {
		return fmt.Errorf("update message status: %w", err)
	}
	return nil
}

func (s *SQLiteStore) QueryMessages(filter MessageFilter) (*model.PagedResult[model.Message], error) {
	var conditions []string
	var args []interface{}

	if filter.Channel != "" {
		conditions = append(conditions, "channel = ?")
		args = append(args, filter.Channel)
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, string(filter.Status))
	}
	if !filter.From.IsZero() {
		conditions = append(conditions, "received_at >= ?")
		args = append(args, filter.From)
	}
	if !filter.To.IsZero() {
		conditions = append(conditions, "received_at <= ?")
		args = append(args, filter.To)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total matching records.
	countQuery := "SELECT COUNT(*) FROM messages " + whereClause
	var total int
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count messages: %w", err)
	}

	// Apply pagination defaults.
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	dataQuery := "SELECT id, source, channel, title, body, extra, status, fail_reason, retry_count, received_at, pushed_at FROM messages " +
		whereClause + " ORDER BY received_at DESC LIMIT ? OFFSET ?"
	dataArgs := append(args, pageSize, offset)

	rows, err := s.db.Query(dataQuery, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var messages []model.Message
	for rows.Next() {
		var msg model.Message
		var extraStr sql.NullString
		var statusStr string
		var pushedAt sql.NullTime

		if err := rows.Scan(
			&msg.ID, &msg.Source, &msg.Channel, &msg.Title, &msg.Body,
			&extraStr, &statusStr, &msg.FailReason, &msg.RetryCount,
			&msg.ReceivedAt, &pushedAt,
		); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}

		msg.Status = model.PushStatus(statusStr)
		if pushedAt.Valid {
			msg.PushedAt = &pushedAt.Time
		}
		if extraStr.Valid && extraStr.String != "" {
			if err := json.Unmarshal([]byte(extraStr.String), &msg.Extra); err != nil {
				return nil, fmt.Errorf("unmarshal extra: %w", err)
			}
		}

		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate messages: %w", err)
	}

	return &model.PagedResult[model.Message]{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Data:     messages,
	}, nil
}

func (s *SQLiteStore) GetChannelStats(from, to time.Time) ([]model.ChannelStats, error) {
	query := `SELECT
		channel,
		COUNT(*) as total,
		SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_count,
		SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_count
	FROM messages
	WHERE received_at >= ? AND received_at <= ?
	GROUP BY channel`

	rows, err := s.db.Query(query, from, to)
	if err != nil {
		return nil, fmt.Errorf("get channel stats: %w", err)
	}
	defer rows.Close()

	var stats []model.ChannelStats
	for rows.Next() {
		var cs model.ChannelStats
		if err := rows.Scan(&cs.Channel, &cs.Total, &cs.SuccessCount, &cs.FailedCount); err != nil {
			return nil, fmt.Errorf("scan channel stats: %w", err)
		}
		if cs.Total > 0 {
			cs.SuccessPercent = float64(cs.SuccessCount) / float64(cs.Total) * 100
			cs.FailedPercent = float64(cs.FailedCount) / float64(cs.Total) * 100
		}
		stats = append(stats, cs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate channel stats: %w", err)
	}

	return stats, nil
}

// --- Reminder operations ---

func (s *SQLiteStore) CreateReminder(r *model.Reminder) error {
	_, err := s.db.Exec(`
		INSERT INTO reminders (id, title, body, channel, trigger_at, repeat_type, repeat_rule, status, next_trigger, last_triggered, fail_reason, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Title, r.Body, r.Channel, r.TriggerAt, string(r.RepeatType), r.RepeatRule,
		r.Status, r.NextTrigger, r.LastTriggered, r.FailReason, r.CreatedAt, r.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create reminder: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdateReminder(r *model.Reminder) error {
	result, err := s.db.Exec(`
		UPDATE reminders
		SET title = ?, body = ?, channel = ?, trigger_at = ?, repeat_type = ?, repeat_rule = ?, status = ?, next_trigger = ?, updated_at = ?
		WHERE id = ?`,
		r.Title, r.Body, r.Channel, r.TriggerAt, string(r.RepeatType), r.RepeatRule, r.Status, r.NextTrigger, time.Now(),
		r.ID,
	)
	if err != nil {
		return fmt.Errorf("update reminder: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update reminder rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("update reminder: not found")
	}
	return nil
}

func (s *SQLiteStore) CancelReminder(id string) error {
	result, err := s.db.Exec(`
		UPDATE reminders SET status = 'cancelled', updated_at = ? WHERE id = ?`,
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("cancel reminder: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("cancel reminder rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("cancel reminder: not found")
	}
	return nil
}

func (s *SQLiteStore) ListReminders(filter ReminderFilter) ([]model.Reminder, error) {
	query := `SELECT id, title, body, channel, trigger_at, repeat_type, repeat_rule, status, next_trigger, last_triggered, fail_reason, created_at, updated_at FROM reminders`
	var args []interface{}

	if filter.Status != "" {
		query += ` WHERE status = ?`
		args = append(args, filter.Status)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	defer rows.Close()

	return scanReminders(rows)
}

func (s *SQLiteStore) GetActiveReminders() ([]model.Reminder, error) {
	rows, err := s.db.Query(`
		SELECT id, title, body, channel, trigger_at, repeat_type, repeat_rule, status, next_trigger, last_triggered, fail_reason, created_at, updated_at
		FROM reminders WHERE status = 'active'
		ORDER BY next_trigger ASC`)
	if err != nil {
		return nil, fmt.Errorf("get active reminders: %w", err)
	}
	defer rows.Close()

	return scanReminders(rows)
}

func (s *SQLiteStore) UpdateNextTrigger(id string, next time.Time) error {
	result, err := s.db.Exec(`
		UPDATE reminders SET next_trigger = ?, updated_at = ? WHERE id = ?`,
		next, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("update next trigger: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update next trigger rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("update next trigger: not found")
	}
	return nil
}

// scanReminders scans rows into a slice of Reminder.
func scanReminders(rows *sql.Rows) ([]model.Reminder, error) {
	var reminders []model.Reminder
	for rows.Next() {
		var r model.Reminder
		var repeatType string
		var body, repeatRule, failReason sql.NullString
		var nextTrigger, lastTriggered sql.NullTime

		err := rows.Scan(
			&r.ID, &r.Title, &body, &r.Channel, &r.TriggerAt,
			&repeatType, &repeatRule, &r.Status, &nextTrigger,
			&lastTriggered, &failReason, &r.CreatedAt, &r.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan reminder: %w", err)
		}

		r.RepeatType = model.RepeatType(repeatType)
		if body.Valid {
			r.Body = body.String
		}
		if repeatRule.Valid {
			r.RepeatRule = repeatRule.String
		}
		if failReason.Valid {
			r.FailReason = failReason.String
		}
		if nextTrigger.Valid {
			r.NextTrigger = &nextTrigger.Time
		}
		if lastTriggered.Valid {
			r.LastTriggered = &lastTriggered.Time
		}

		reminders = append(reminders, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan reminders: %w", err)
	}
	return reminders, nil
}



// --- Device operations ---

func (s *SQLiteStore) CreateDevice(d *model.Device) error {
	deviceType := d.Type
	if deviceType == "" {
		deviceType = model.DeviceTypeBark
	}
	_, err := s.db.Exec(`
		INSERT INTO devices (id, name, device_key, type, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.Name, d.DeviceKey, deviceType, boolToInt(d.IsDefault), d.CreatedAt, d.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create device: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdateDevice(d *model.Device) error {
	result, err := s.db.Exec(`
		UPDATE devices SET name = ?, device_key = ?, type = ?, is_default = ?, updated_at = ?
		WHERE id = ?`,
		d.Name, d.DeviceKey, d.Type, boolToInt(d.IsDefault), time.Now(), d.ID,
	)
	if err != nil {
		return fmt.Errorf("update device: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("update device: not found")
	}
	return nil
}

func (s *SQLiteStore) DeleteDevice(id string) error {
	result, err := s.db.Exec(`DELETE FROM devices WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete device: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("delete device: not found")
	}
	return nil
}

func (s *SQLiteStore) ListDevices() ([]model.Device, error) {
	rows, err := s.db.Query(`
		SELECT id, name, device_key, type, is_default, created_at, updated_at
		FROM devices ORDER BY is_default DESC, created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()

	var devices []model.Device
	for rows.Next() {
		var d model.Device
		var isDefault int
		if err := rows.Scan(&d.ID, &d.Name, &d.DeviceKey, &d.Type, &isDefault, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		d.IsDefault = isDefault != 0
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func (s *SQLiteStore) GetDefaultDevice() (*model.Device, error) {
	var d model.Device
	var isDefault int
	err := s.db.QueryRow(`
		SELECT id, name, device_key, type, is_default, created_at, updated_at
		FROM devices WHERE is_default = 1 LIMIT 1`).
		Scan(&d.ID, &d.Name, &d.DeviceKey, &d.Type, &isDefault, &d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get default device: %w", err)
	}
	d.IsDefault = true
	return &d, nil
}

func (s *SQLiteStore) SetDefaultDevice(id string) error {
	// Clear all defaults first
	if _, err := s.db.Exec(`UPDATE devices SET is_default = 0`); err != nil {
		return fmt.Errorf("clear default device: %w", err)
	}
	// Set the new default
	result, err := s.db.Exec(`UPDATE devices SET is_default = 1, updated_at = ? WHERE id = ?`, time.Now(), id)
	if err != nil {
		return fmt.Errorf("set default device: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("set default device: not found")
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// GetDeviceByKey looks up a device by its key and type.
func (s *SQLiteStore) GetDeviceByKey(deviceKey string, deviceType string) (*model.Device, error) {
	var d model.Device
	var isDefault int
	err := s.db.QueryRow(`
		SELECT id, name, device_key, type, is_default, created_at, updated_at
		FROM devices WHERE device_key = ? AND type = ?`, deviceKey, deviceType).
		Scan(&d.ID, &d.Name, &d.DeviceKey, &d.Type, &isDefault, &d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get device by key: %w", err)
	}
	d.IsDefault = isDefault != 0
	return &d, nil
}

// CountDevicesByType returns the number of devices with the given type.
func (s *SQLiteStore) CountDevicesByType(deviceType string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM devices WHERE type = ?`, deviceType).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count devices by type: %w", err)
	}
	return count, nil
}

// --- Channel operations ---

func (s *SQLiteStore) CreateChannel(ch *model.Channel) error {
	deviceKeysJSON, _ := json.Marshal(ch.DeviceKeys)
	_, err := s.db.Exec(`
		INSERT INTO channels (id, name, sound, grp, icon, level, device_keys, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ch.ID, ch.Name, ch.Sound, ch.Group, ch.Icon, ch.Level, string(deviceKeysJSON), ch.CreatedAt, ch.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create channel: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdateChannel(ch *model.Channel) error {
	deviceKeysJSON, _ := json.Marshal(ch.DeviceKeys)
	result, err := s.db.Exec(`
		UPDATE channels SET name = ?, sound = ?, grp = ?, icon = ?, level = ?, device_keys = ?, updated_at = ?
		WHERE id = ?`,
		ch.Name, ch.Sound, ch.Group, ch.Icon, ch.Level, string(deviceKeysJSON), time.Now(), ch.ID,
	)
	if err != nil {
		return fmt.Errorf("update channel: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("update channel: not found")
	}
	return nil
}

func (s *SQLiteStore) DeleteChannel(id string) error {
	result, err := s.db.Exec(`DELETE FROM channels WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete channel: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("delete channel: not found")
	}
	return nil
}

func (s *SQLiteStore) ListChannels() ([]model.Channel, error) {
	rows, err := s.db.Query(`
		SELECT id, name, sound, grp, icon, level, device_keys, created_at, updated_at
		FROM channels ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()

	var channels []model.Channel
	for rows.Next() {
		var ch model.Channel
		var deviceKeysJSON string
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.Sound, &ch.Group, &ch.Icon, &ch.Level, &deviceKeysJSON, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}
		if deviceKeysJSON != "" {
			_ = json.Unmarshal([]byte(deviceKeysJSON), &ch.DeviceKeys)
		}
		if ch.DeviceKeys == nil {
			ch.DeviceKeys = []string{}
		}
		channels = append(channels, ch)
	}
	return channels, rows.Err()
}

func (s *SQLiteStore) GetChannelByName(name string) (*model.Channel, error) {
	var ch model.Channel
	var deviceKeysJSON string
	err := s.db.QueryRow(`
		SELECT id, name, sound, grp, icon, level, device_keys, created_at, updated_at
		FROM channels WHERE name = ?`, name).
		Scan(&ch.ID, &ch.Name, &ch.Sound, &ch.Group, &ch.Icon, &ch.Level, &deviceKeysJSON, &ch.CreatedAt, &ch.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get channel by name: %w", err)
	}
	if deviceKeysJSON != "" {
		_ = json.Unmarshal([]byte(deviceKeysJSON), &ch.DeviceKeys)
	}
	if ch.DeviceKeys == nil {
		ch.DeviceKeys = []string{}
	}
	return &ch, nil
}

// --- Auth operations ---

func (s *SQLiteStore) CreateUser(u *model.User) error {
	_, err := s.db.Exec(`
		INSERT INTO users (id, username, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`,
		u.ID, u.Username, u.PasswordHash, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetUserByUsername(username string) (*model.User, error) {
	var u model.User
	err := s.db.QueryRow(`
		SELECT id, username, password_hash, created_at, updated_at
		FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	return &u, nil
}

func (s *SQLiteStore) GetUserCount() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get user count: %w", err)
	}
	return count, nil
}

func (s *SQLiteStore) UpdateUserPassword(id string, passwordHash string) error {
	result, err := s.db.Exec(`
		UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		passwordHash, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("update user password: not found")
	}
	return nil
}

// --- API Key operations ---

func (s *SQLiteStore) CreateAPIKey(key *model.APIKey) error {
	_, err := s.db.Exec(`
		INSERT INTO api_keys (id, name, key_hash, prefix, user_id, created_at, last_used)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		key.ID, key.Name, key.KeyHash, key.Prefix, key.UserID, key.CreatedAt, key.LastUsed,
	)
	if err != nil {
		return fmt.Errorf("create api key: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListAPIKeys(userID string) ([]model.APIKey, error) {
	rows, err := s.db.Query(`
		SELECT id, name, prefix, user_id, created_at, last_used
		FROM api_keys WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var keys []model.APIKey
	for rows.Next() {
		var k model.APIKey
		var lastUsed sql.NullTime
		if err := rows.Scan(&k.ID, &k.Name, &k.Prefix, &k.UserID, &k.CreatedAt, &lastUsed); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		if lastUsed.Valid {
			k.LastUsed = &lastUsed.Time
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *SQLiteStore) DeleteAPIKey(id string) error {
	result, err := s.db.Exec(`DELETE FROM api_keys WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("delete api key: not found")
	}
	return nil
}

func (s *SQLiteStore) GetAPIKeyByHash(keyHash string) (*model.APIKey, error) {
	var k model.APIKey
	var lastUsed sql.NullTime
	err := s.db.QueryRow(`
		SELECT id, name, key_hash, prefix, user_id, created_at, last_used
		FROM api_keys WHERE key_hash = ?`, keyHash).
		Scan(&k.ID, &k.Name, &k.KeyHash, &k.Prefix, &k.UserID, &k.CreatedAt, &lastUsed)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get api key by hash: %w", err)
	}
	if lastUsed.Valid {
		k.LastUsed = &lastUsed.Time
	}
	return &k, nil
}

func (s *SQLiteStore) UpdateAPIKeyLastUsed(id string) error {
	_, err := s.db.Exec(`UPDATE api_keys SET last_used = ? WHERE id = ?`, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update api key last used: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ValidateAPIKey(rawKey string) (*model.APIKey, error) {
	rows, err := s.db.Query(`SELECT id, name, key_hash, prefix, user_id, created_at, last_used FROM api_keys`)
	if err != nil {
		return nil, fmt.Errorf("validate api key: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var k model.APIKey
		var lastUsed sql.NullTime
		if err := rows.Scan(&k.ID, &k.Name, &k.KeyHash, &k.Prefix, &k.UserID, &k.CreatedAt, &lastUsed); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		if lastUsed.Valid {
			k.LastUsed = &lastUsed.Time
		}
		if err := bcrypt.CompareHashAndPassword([]byte(k.KeyHash), []byte(rawKey)); err == nil {
			// Match found - update last_used
			_ = s.UpdateAPIKeyLastUsed(k.ID)
			return &k, nil
		}
	}
	return nil, nil
}

// --- Settings operations ---

func (s *SQLiteStore) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get setting: %w", err)
	}
	return value, nil
}

func (s *SQLiteStore) SetSetting(key string, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?`, key, value, value)
	if err != nil {
		return fmt.Errorf("set setting: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetSettings(prefix string) (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM settings WHERE key LIKE ?`, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		result[k] = v
	}
	return result, rows.Err()
}
