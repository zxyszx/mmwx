package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	SubscribeTypeCreate = "create"
	SubscribeTypeImport = "import"
	SubscribeTypeUpload = "upload"
)

func parseSubscribeFileJSONFields(file *SubscribeFile, tagsJSON, nodeIDsJSON, ruleIDsJSON, scriptIDsJSON string) {
	if tagsJSON != "" && tagsJSON != "[]" {
		_ = json.Unmarshal([]byte(tagsJSON), &file.SelectedTags)
	}
	if nodeIDsJSON != "" && nodeIDsJSON != "[]" {
		_ = json.Unmarshal([]byte(nodeIDsJSON), &file.SelectedNodeIDs)
	}
	if ruleIDsJSON != "" && ruleIDsJSON != "[]" {
		_ = json.Unmarshal([]byte(ruleIDsJSON), &file.SelectedCustomRuleIDs)
	}
	if scriptIDsJSON != "" && scriptIDsJSON != "[]" {
		_ = json.Unmarshal([]byte(scriptIDsJSON), &file.SelectedOverrideScriptIDs)
	}
}

func serializeInt64Slice(ids []int64) string {
	if len(ids) == 0 {
		return "[]"
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// ListSubscribeFiles returns all subscribe files ordered by creation time.
func (r *TrafficRepository) ListSubscribeFiles(ctx context.Context) ([]SubscribeFile, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("traffic repository not initialized")
	}

	rows, err := r.db.QueryContext(ctx, `SELECT id, name, COALESCE(description, ''), url, type, filename, COALESCE(file_short_code, ''), COALESCE(custom_short_code, ''), COALESCE(auto_sync_custom_rules, 0), COALESCE(template_filename, ''), COALESCE(selected_tags, '[]'), COALESCE(selected_node_ids, '[]'), COALESCE(selected_custom_rule_ids, '[]'), COALESCE(selected_override_script_ids, '[]'), expire_at, COALESCE(raw_output, 0), COALESCE(sort_order, 0), created_at, updated_at, traffic_limit, COALESCE(stats_server_ids, '') FROM subscribe_files ORDER BY sort_order ASC, created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list subscribe files: %w", err)
	}
	defer rows.Close()

	var files []SubscribeFile
	for rows.Next() {
		var file SubscribeFile
		var autoSync int
		var rawOutput int
		var expireAt sql.NullTime
		var selectedTagsJSON, selectedNodeIDsJSON, selectedRuleIDsJSON, selectedScriptIDsJSON string
		var trafficLimit sql.NullFloat64
		if err := rows.Scan(&file.ID, &file.Name, &file.Description, &file.URL, &file.Type, &file.Filename, &file.FileShortCode, &file.CustomShortCode, &autoSync, &file.TemplateFilename, &selectedTagsJSON, &selectedNodeIDsJSON, &selectedRuleIDsJSON, &selectedScriptIDsJSON, &expireAt, &rawOutput, &file.SortOrder, &file.CreatedAt, &file.UpdatedAt, &trafficLimit, &file.StatsServerIDs); err != nil {
			return nil, fmt.Errorf("scan subscribe file: %w", err)
		}
		file.AutoSyncCustomRules = autoSync != 0
		file.RawOutput = rawOutput != 0
		if expireAt.Valid {
			file.ExpireAt = &expireAt.Time
		}
		if trafficLimit.Valid {
			v := trafficLimit.Float64
			file.TrafficLimit = &v
		}
		parseSubscribeFileJSONFields(&file, selectedTagsJSON, selectedNodeIDsJSON, selectedRuleIDsJSON, selectedScriptIDsJSON)
		files = append(files, file)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscribe files: %w", err)
	}

	return files, nil
}

// GetSubscribeFileByID retrieves a subscribe file by ID.
func (r *TrafficRepository) GetSubscribeFileByID(ctx context.Context, id int64) (SubscribeFile, error) {
	var file SubscribeFile
	if r == nil || r.db == nil {
		return file, errors.New("traffic repository not initialized")
	}

	if id <= 0 {
		return file, errors.New("subscribe file id is required")
	}

	row := r.db.QueryRowContext(ctx, `SELECT id, name, COALESCE(description, ''), url, type, filename, COALESCE(file_short_code, ''), COALESCE(custom_short_code, ''), COALESCE(auto_sync_custom_rules, 0), COALESCE(template_filename, ''), COALESCE(selected_tags, '[]'), COALESCE(selected_node_ids, '[]'), COALESCE(selected_custom_rule_ids, '[]'), COALESCE(selected_override_script_ids, '[]'), expire_at, COALESCE(raw_output, 0), COALESCE(sort_order, 0), created_at, updated_at, traffic_limit, COALESCE(stats_server_ids, '') FROM subscribe_files WHERE id = ? LIMIT 1`, id)
	var autoSync int
	var rawOutput int
	var expireAt sql.NullTime
	var selectedTagsJSON, selectedNodeIDsJSON, selectedRuleIDsJSON, selectedScriptIDsJSON string
	var trafficLimit sql.NullFloat64
	if err := row.Scan(&file.ID, &file.Name, &file.Description, &file.URL, &file.Type, &file.Filename, &file.FileShortCode, &file.CustomShortCode, &autoSync, &file.TemplateFilename, &selectedTagsJSON, &selectedNodeIDsJSON, &selectedRuleIDsJSON, &selectedScriptIDsJSON, &expireAt, &rawOutput, &file.SortOrder, &file.CreatedAt, &file.UpdatedAt, &trafficLimit, &file.StatsServerIDs); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return file, ErrSubscribeFileNotFound
		}
		return file, fmt.Errorf("get subscribe file: %w", err)
	}
	file.AutoSyncCustomRules = autoSync != 0
	file.RawOutput = rawOutput != 0
	if expireAt.Valid {
		file.ExpireAt = &expireAt.Time
	}
	if trafficLimit.Valid {
		v := trafficLimit.Float64
		file.TrafficLimit = &v
	}
	parseSubscribeFileJSONFields(&file, selectedTagsJSON, selectedNodeIDsJSON, selectedRuleIDsJSON, selectedScriptIDsJSON)

	return file, nil
}

// GetSubscribeFileByName retrieves a subscribe file by name.
func (r *TrafficRepository) GetSubscribeFileByName(ctx context.Context, name string) (SubscribeFile, error) {
	var file SubscribeFile
	if r == nil || r.db == nil {
		return file, errors.New("traffic repository not initialized")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return file, errors.New("subscribe file name is required")
	}

	row := r.db.QueryRowContext(ctx, `SELECT id, name, COALESCE(description, ''), url, type, filename, COALESCE(file_short_code, ''), COALESCE(custom_short_code, ''), COALESCE(auto_sync_custom_rules, 0), COALESCE(template_filename, ''), COALESCE(selected_tags, '[]'), COALESCE(selected_node_ids, '[]'), COALESCE(selected_custom_rule_ids, '[]'), COALESCE(selected_override_script_ids, '[]'), expire_at, COALESCE(raw_output, 0), COALESCE(sort_order, 0), created_at, updated_at, traffic_limit, COALESCE(stats_server_ids, '') FROM subscribe_files WHERE name = ? LIMIT 1`, name)
	var autoSync int
	var rawOutput int
	var expireAt sql.NullTime
	var selectedTagsJSON, selectedNodeIDsJSON, selectedRuleIDsJSON, selectedScriptIDsJSON string
	var trafficLimit sql.NullFloat64
	if err := row.Scan(&file.ID, &file.Name, &file.Description, &file.URL, &file.Type, &file.Filename, &file.FileShortCode, &file.CustomShortCode, &autoSync, &file.TemplateFilename, &selectedTagsJSON, &selectedNodeIDsJSON, &selectedRuleIDsJSON, &selectedScriptIDsJSON, &expireAt, &rawOutput, &file.SortOrder, &file.CreatedAt, &file.UpdatedAt, &trafficLimit, &file.StatsServerIDs); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return file, ErrSubscribeFileNotFound
		}
		return file, fmt.Errorf("get subscribe file by name: %w", err)
	}
	file.AutoSyncCustomRules = autoSync != 0
	file.RawOutput = rawOutput != 0
	if expireAt.Valid {
		file.ExpireAt = &expireAt.Time
	}
	if trafficLimit.Valid {
		v := trafficLimit.Float64
		file.TrafficLimit = &v
	}
	parseSubscribeFileJSONFields(&file, selectedTagsJSON, selectedNodeIDsJSON, selectedRuleIDsJSON, selectedScriptIDsJSON)

	return file, nil
}

// GetSubscribeFileByFilename retrieves a subscribe file by filename.
func (r *TrafficRepository) GetSubscribeFileByFilename(ctx context.Context, filename string) (SubscribeFile, error) {
	var file SubscribeFile
	if r == nil || r.db == nil {
		return file, errors.New("traffic repository not initialized")
	}

	filename = strings.TrimSpace(filename)
	if filename == "" {
		return file, errors.New("subscribe file filename is required")
	}

	row := r.db.QueryRowContext(ctx, `SELECT id, name, COALESCE(description, ''), url, type, filename, COALESCE(file_short_code, ''), COALESCE(custom_short_code, ''), COALESCE(auto_sync_custom_rules, 0), COALESCE(template_filename, ''), COALESCE(selected_tags, '[]'), COALESCE(selected_node_ids, '[]'), COALESCE(selected_custom_rule_ids, '[]'), COALESCE(selected_override_script_ids, '[]'), expire_at, COALESCE(raw_output, 0), COALESCE(sort_order, 0), created_at, updated_at, traffic_limit, COALESCE(stats_server_ids, '') FROM subscribe_files WHERE filename = ? LIMIT 1`, filename)
	var autoSync int
	var rawOutput int
	var expireAt sql.NullTime
	var selectedTagsJSON, selectedNodeIDsJSON, selectedRuleIDsJSON, selectedScriptIDsJSON string
	var trafficLimit sql.NullFloat64
	if err := row.Scan(&file.ID, &file.Name, &file.Description, &file.URL, &file.Type, &file.Filename, &file.FileShortCode, &file.CustomShortCode, &autoSync, &file.TemplateFilename, &selectedTagsJSON, &selectedNodeIDsJSON, &selectedRuleIDsJSON, &selectedScriptIDsJSON, &expireAt, &rawOutput, &file.SortOrder, &file.CreatedAt, &file.UpdatedAt, &trafficLimit, &file.StatsServerIDs); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return file, ErrSubscribeFileNotFound
		}
		return file, fmt.Errorf("get subscribe file by filename: %w", err)
	}
	file.AutoSyncCustomRules = autoSync != 0
	file.RawOutput = rawOutput != 0
	if expireAt.Valid {
		file.ExpireAt = &expireAt.Time
	}
	if trafficLimit.Valid {
		v := trafficLimit.Float64
		file.TrafficLimit = &v
	}
	parseSubscribeFileJSONFields(&file, selectedTagsJSON, selectedNodeIDsJSON, selectedRuleIDsJSON, selectedScriptIDsJSON)

	return file, nil
}

// CreateSubscribeFile inserts a new subscribe file record.
func (r *TrafficRepository) CreateSubscribeFile(ctx context.Context, file SubscribeFile) (SubscribeFile, error) {
	if r == nil || r.db == nil {
		return SubscribeFile{}, errors.New("traffic repository not initialized")
	}

	file.Name = strings.TrimSpace(file.Name)
	file.Description = strings.TrimSpace(file.Description)
	file.URL = strings.TrimSpace(file.URL)
	file.Type = strings.ToLower(strings.TrimSpace(file.Type))
	file.Filename = strings.TrimSpace(file.Filename)

	if file.Name == "" {
		return SubscribeFile{}, errors.New("subscribe file name is required")
	}
	if file.Type != SubscribeTypeCreate && file.Type != SubscribeTypeImport && file.Type != SubscribeTypeUpload {
		return SubscribeFile{}, errors.New("invalid subscribe file type")
	}
	// URL只对import类型必填，upload类型可以为空
	if (file.Type == SubscribeTypeImport) && file.URL == "" {
		return SubscribeFile{}, errors.New("subscribe file url is required")
	}
	if file.Filename == "" {
		return SubscribeFile{}, errors.New("subscribe file filename is required")
	}

	// Generate file short code with retry logic for collision handling
	const maxRetries = 10
	var expireAt any
	if file.ExpireAt != nil {
		expireAt = *file.ExpireAt
	}
	// Serialize JSON fields
	selectedTagsJSON := "[]"
	if len(file.SelectedTags) > 0 {
		if tagsBytes, err := json.Marshal(file.SelectedTags); err == nil {
			selectedTagsJSON = string(tagsBytes)
		}
	}
	selectedNodeIDsJSON := serializeInt64Slice(file.SelectedNodeIDs)
	selectedRuleIDsJSON := serializeInt64Slice(file.SelectedCustomRuleIDs)
	selectedScriptIDsJSON := serializeInt64Slice(file.SelectedOverrideScriptIDs)
	for i := 0; i < maxRetries; i++ {
		newFileShortCode, err := generateFileShortCode()
		if err != nil {
			return SubscribeFile{}, fmt.Errorf("generate file short code: %w", err)
		}

		// Default auto_sync_custom_rules to 1 (enabled) for new subscribe files
		// template_filename 默认为空，创建时不绑定模板
		var rawOutputInt int
		if file.RawOutput {
			rawOutputInt = 1
		}
		res, err := r.db.ExecContext(ctx, `INSERT INTO subscribe_files (name, description, url, type, filename, file_short_code, auto_sync_custom_rules, template_filename, selected_tags, selected_node_ids, selected_custom_rule_ids, selected_override_script_ids, expire_at, raw_output, traffic_limit, stats_server_ids) VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			file.Name, file.Description, file.URL, file.Type, file.Filename, newFileShortCode, file.TemplateFilename, selectedTagsJSON, selectedNodeIDsJSON, selectedRuleIDsJSON, selectedScriptIDsJSON, expireAt, rawOutputInt, file.TrafficLimit, file.StatsServerIDs)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") && strings.Contains(strings.ToLower(err.Error()), "file_short_code") {
				// File short code collision, retry
				continue
			}
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				return SubscribeFile{}, ErrSubscribeFileExists
			}
			return SubscribeFile{}, fmt.Errorf("create subscribe file: %w", err)
		}

		id, err := res.LastInsertId()
		if err != nil {
			return SubscribeFile{}, fmt.Errorf("fetch subscribe file id: %w", err)
		}

		return r.GetSubscribeFileByID(ctx, id)
	}

	return SubscribeFile{}, errors.New("failed to generate unique file short code after retries")
}

// UpdateSubscribeFile updates an existing subscribe file record.
func (r *TrafficRepository) UpdateSubscribeFile(ctx context.Context, file SubscribeFile) (SubscribeFile, error) {
	if r == nil || r.db == nil {
		return SubscribeFile{}, errors.New("traffic repository not initialized")
	}

	if file.ID <= 0 {
		return SubscribeFile{}, errors.New("subscribe file id is required")
	}

	file.Name = strings.TrimSpace(file.Name)
	file.Description = strings.TrimSpace(file.Description)
	file.URL = strings.TrimSpace(file.URL)
	file.Type = strings.ToLower(strings.TrimSpace(file.Type))
	file.Filename = strings.TrimSpace(file.Filename)

	if file.Name == "" {
		return SubscribeFile{}, errors.New("subscribe file name is required")
	}
	if file.Type != SubscribeTypeCreate && file.Type != SubscribeTypeImport && file.Type != SubscribeTypeUpload {
		return SubscribeFile{}, errors.New("invalid subscribe file type")
	}
	// URL只对import类型必填，upload类型可以为空
	if (file.Type == SubscribeTypeImport) && file.URL == "" {
		return SubscribeFile{}, errors.New("subscribe file url is required")
	}
	if file.Filename == "" {
		return SubscribeFile{}, errors.New("subscribe file filename is required")
	}

	var autoSyncInt int
	if file.AutoSyncCustomRules {
		autoSyncInt = 1
	}
	var expireAt any
	if file.ExpireAt != nil {
		expireAt = *file.ExpireAt
	}
	// Serialize JSON fields
	selectedTagsJSON := "[]"
	if len(file.SelectedTags) > 0 {
		if tagsBytes, err := json.Marshal(file.SelectedTags); err == nil {
			selectedTagsJSON = string(tagsBytes)
		}
	}
	selectedNodeIDsJSON := serializeInt64Slice(file.SelectedNodeIDs)
	selectedRuleIDsJSON := serializeInt64Slice(file.SelectedCustomRuleIDs)
	selectedScriptIDsJSON := serializeInt64Slice(file.SelectedOverrideScriptIDs)
	var rawOutputInt int
	if file.RawOutput {
		rawOutputInt = 1
	}
	res, err := r.db.ExecContext(ctx, `UPDATE subscribe_files SET name = ?, description = ?, url = ?, type = ?, filename = ?, auto_sync_custom_rules = ?, template_filename = ?, selected_tags = ?, selected_node_ids = ?, selected_custom_rule_ids = ?, selected_override_script_ids = ?, custom_short_code = ?, expire_at = ?, raw_output = ?, traffic_limit = ?, stats_server_ids = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		file.Name, file.Description, file.URL, file.Type, file.Filename, autoSyncInt, file.TemplateFilename, selectedTagsJSON, selectedNodeIDsJSON, selectedRuleIDsJSON, selectedScriptIDsJSON, file.CustomShortCode, expireAt, rawOutputInt, file.TrafficLimit, file.StatsServerIDs, file.ID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return SubscribeFile{}, ErrSubscribeFileExists
		}
		return SubscribeFile{}, fmt.Errorf("update subscribe file: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return SubscribeFile{}, fmt.Errorf("subscribe file update rows affected: %w", err)
	}
	if affected == 0 {
		return SubscribeFile{}, ErrSubscribeFileNotFound
	}

	return r.GetSubscribeFileByID(ctx, file.ID)
}

// ReorderSubscribeFiles updates sort_order for subscribe files based on the given ID order.
func (r *TrafficRepository) ReorderSubscribeFiles(ctx context.Context, ids []int64) error {
	if r == nil || r.db == nil {
		return errors.New("traffic repository not initialized")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin reorder tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `UPDATE subscribe_files SET sort_order = ? WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("prepare reorder stmt: %w", err)
	}
	defer stmt.Close()

	for i, id := range ids {
		if _, err := stmt.ExecContext(ctx, i, id); err != nil {
			return fmt.Errorf("update sort_order for id %d: %w", id, err)
		}
	}

	return tx.Commit()
}

// DeleteSubscribeFile removes a subscribe file record.
func (r *TrafficRepository) DeleteSubscribeFile(ctx context.Context, id int64) error {
	if r == nil || r.db == nil {
		return errors.New("traffic repository not initialized")
	}

	if id <= 0 {
		return errors.New("subscribe file id is required")
	}

	// Start a transaction to ensure both deletions succeed or fail together
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// First, delete related user_subscriptions records
	_, err = tx.ExecContext(ctx, `DELETE FROM user_subscriptions WHERE subscription_id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete user subscriptions: %w", err)
	}

	// Then, delete the subscribe file
	res, err := tx.ExecContext(ctx, `DELETE FROM subscribe_files WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete subscribe file: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("subscribe file delete rows affected: %w", err)
	}
	if affected == 0 {
		return ErrSubscribeFileNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetSubscribeFilesByTemplate 获取绑定了指定模板的所有订阅文件
// templateFilename 为模板文件名（如 "templates/my-template.yaml"）
func (r *TrafficRepository) GetSubscribeFilesByTemplate(ctx context.Context, templateFilename string) ([]SubscribeFile, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("traffic repository not initialized")
	}

	templateFilename = strings.TrimSpace(templateFilename)
	if templateFilename == "" {
		return nil, errors.New("template filename is required")
	}

	const query = `SELECT id, name, COALESCE(description, ''), url, type, filename, COALESCE(file_short_code, ''), COALESCE(custom_short_code, ''), COALESCE(auto_sync_custom_rules, 0), COALESCE(template_filename, ''), COALESCE(selected_tags, '[]'), COALESCE(selected_node_ids, '[]'), COALESCE(selected_custom_rule_ids, '[]'), COALESCE(selected_override_script_ids, '[]'), expire_at, COALESCE(raw_output, 0), COALESCE(sort_order, 0), created_at, updated_at, traffic_limit, COALESCE(stats_server_ids, '')
		FROM subscribe_files
		WHERE template_filename = ?
		ORDER BY sort_order ASC, created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, templateFilename)
	if err != nil {
		return nil, fmt.Errorf("get subscribe files by template: %w", err)
	}
	defer rows.Close()

	var files []SubscribeFile
	for rows.Next() {
		var file SubscribeFile
		var autoSync int
		var rawOutput int
		var expireAt sql.NullTime
		var selectedTagsJSON, selectedNodeIDsJSON, selectedRuleIDsJSON, selectedScriptIDsJSON string
		var trafficLimit sql.NullFloat64
		if err := rows.Scan(&file.ID, &file.Name, &file.Description, &file.URL, &file.Type, &file.Filename, &file.FileShortCode, &file.CustomShortCode, &autoSync, &file.TemplateFilename, &selectedTagsJSON, &selectedNodeIDsJSON, &selectedRuleIDsJSON, &selectedScriptIDsJSON, &expireAt, &rawOutput, &file.SortOrder, &file.CreatedAt, &file.UpdatedAt, &trafficLimit, &file.StatsServerIDs); err != nil {
			return nil, fmt.Errorf("scan subscribe file: %w", err)
		}
		file.AutoSyncCustomRules = autoSync != 0
		file.RawOutput = rawOutput != 0
		if expireAt.Valid {
			file.ExpireAt = &expireAt.Time
		}
		if trafficLimit.Valid {
			v := trafficLimit.Float64
			file.TrafficLimit = &v
		}
		parseSubscribeFileJSONFields(&file, selectedTagsJSON, selectedNodeIDsJSON, selectedRuleIDsJSON, selectedScriptIDsJSON)
		files = append(files, file)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscribe files: %w", err)
	}

	return files, nil
}

// GetSubscribeFilesWithTemplate 获取所有绑定了模板的订阅文件
func (r *TrafficRepository) GetSubscribeFilesWithTemplate(ctx context.Context) ([]SubscribeFile, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("traffic repository not initialized")
	}

	const query = `SELECT id, name, COALESCE(description, ''), url, type, filename, COALESCE(file_short_code, ''), COALESCE(custom_short_code, ''), COALESCE(auto_sync_custom_rules, 0), COALESCE(template_filename, ''), COALESCE(selected_tags, '[]'), COALESCE(selected_node_ids, '[]'), COALESCE(selected_custom_rule_ids, '[]'), COALESCE(selected_override_script_ids, '[]'), expire_at, COALESCE(raw_output, 0), COALESCE(sort_order, 0), created_at, updated_at, traffic_limit, COALESCE(stats_server_ids, '')
		FROM subscribe_files
		WHERE template_filename IS NOT NULL AND template_filename != ''
		ORDER BY sort_order ASC, created_at DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get subscribe files with template: %w", err)
	}
	defer rows.Close()

	var files []SubscribeFile
	for rows.Next() {
		var file SubscribeFile
		var autoSync int
		var rawOutput int
		var expireAt sql.NullTime
		var selectedTagsJSON, selectedNodeIDsJSON, selectedRuleIDsJSON, selectedScriptIDsJSON string
		var trafficLimit sql.NullFloat64
		if err := rows.Scan(&file.ID, &file.Name, &file.Description, &file.URL, &file.Type, &file.Filename, &file.FileShortCode, &file.CustomShortCode, &autoSync, &file.TemplateFilename, &selectedTagsJSON, &selectedNodeIDsJSON, &selectedRuleIDsJSON, &selectedScriptIDsJSON, &expireAt, &rawOutput, &file.SortOrder, &file.CreatedAt, &file.UpdatedAt, &trafficLimit, &file.StatsServerIDs); err != nil {
			return nil, fmt.Errorf("scan subscribe file: %w", err)
		}
		file.AutoSyncCustomRules = autoSync != 0
		file.RawOutput = rawOutput != 0
		if expireAt.Valid {
			file.ExpireAt = &expireAt.Time
		}
		if trafficLimit.Valid {
			v := trafficLimit.Float64
			file.TrafficLimit = &v
		}
		parseSubscribeFileJSONFields(&file, selectedTagsJSON, selectedNodeIDsJSON, selectedRuleIDsJSON, selectedScriptIDsJSON)
		files = append(files, file)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscribe files with template: %w", err)
	}

	return files, nil
}

// CleanupStatsServerIDs removes invalid server IDs from all subscribe files' stats_server_ids.
func (r *TrafficRepository) CleanupStatsServerIDs(ctx context.Context, validServerIDs []string) error {
	if r == nil || r.db == nil {
		return errors.New("traffic repository not initialized")
	}

	validSet := make(map[string]struct{}, len(validServerIDs))
	for _, id := range validServerIDs {
		validSet[id] = struct{}{}
	}

	rows, err := r.db.QueryContext(ctx, `SELECT id, stats_server_ids FROM subscribe_files WHERE stats_server_ids != ''`)
	if err != nil {
		return fmt.Errorf("query stats_server_ids: %w", err)
	}
	defer rows.Close()

	type record struct {
		id  int64
		ids string
	}
	var records []record
	for rows.Next() {
		var rec record
		if err := rows.Scan(&rec.id, &rec.ids); err != nil {
			return fmt.Errorf("scan stats_server_ids: %w", err)
		}
		records = append(records, rec)
	}

	for _, rec := range records {
		parts := strings.Split(rec.ids, ",")
		var kept []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if _, ok := validSet[p]; ok {
				kept = append(kept, p)
			}
		}
		newVal := strings.Join(kept, ",")
		if newVal != rec.ids {
			if _, err := r.db.ExecContext(ctx, `UPDATE subscribe_files SET stats_server_ids = ? WHERE id = ?`, newVal, rec.id); err != nil {
				return fmt.Errorf("update stats_server_ids for file %d: %w", rec.id, err)
			}
		}
	}

	return nil
}

// ClearAllStatsServerIDs resets stats_server_ids for all subscribe files.
func (r *TrafficRepository) ClearAllStatsServerIDs(ctx context.Context) error {
	if r == nil || r.db == nil {
		return errors.New("traffic repository not initialized")
	}
	_, err := r.db.ExecContext(ctx, `UPDATE subscribe_files SET stats_server_ids = '' WHERE stats_server_ids != ''`)
	if err != nil {
		return fmt.Errorf("clear all stats_server_ids: %w", err)
	}
	return nil
}
