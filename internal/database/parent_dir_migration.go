package database

import (
	"fmt"
	"path"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/unkmonster/tmd/internal/utils"
)

type ParentDirMigrationOptions struct {
	FromRoot string
	ToRoot   string
	DryRun   bool
}

type ParentDirMigrationSample struct {
	Table string
	ID    int64
	From  string
	To    string
}

type ParentDirMigrationResult struct {
	BackupPath          string
	UserEntitiesTotal   int
	UserEntitiesUpdated int
	LstEntitiesTotal    int
	LstEntitiesUpdated  int
	Samples             []ParentDirMigrationSample
}

type parentDirOwnerRow struct {
	ID        int64  `db:"id"`
	OwnerID   int64  `db:"owner_id"`
	ParentDir string `db:"parent_dir"`
}

type portablePathStyle int

const (
	portablePathStylePosix portablePathStyle = iota
	portablePathStyleWindows
)

type portablePath struct {
	style      portablePathStyle
	absolute   bool
	volumeID   string
	rootPrefix string
	segments   []string
}

func MigrateParentDirsInSQLiteFile(dbPath string, opts ParentDirMigrationOptions) (*ParentDirMigrationResult, error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, fmt.Errorf("db path is required")
	}

	exists, err := utils.PathExists(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect database path %q: %w", dbPath, err)
	}
	if !exists {
		return nil, fmt.Errorf("database file %q does not exist", dbPath)
	}

	fromRoot, err := parsePortablePath(opts.FromRoot)
	if err != nil {
		return nil, fmt.Errorf("invalid from-root %q: %w", opts.FromRoot, err)
	}
	if !fromRoot.absolute {
		return nil, fmt.Errorf("from-root %q must be absolute", opts.FromRoot)
	}

	toRoot, err := parsePortablePath(opts.ToRoot)
	if err != nil {
		return nil, fmt.Errorf("invalid to-root %q: %w", opts.ToRoot, err)
	}
	if !toRoot.absolute {
		return nil, fmt.Errorf("to-root %q must be absolute", opts.ToRoot)
	}

	db, err := Connect(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return nil, fmt.Errorf("failed to checkpoint database before migration: %w", err)
	}

	result := &ParentDirMigrationResult{}

	userUpdates, err := collectParentDirUpdates(
		db,
		`SELECT id, user_id AS owner_id, parent_dir FROM user_entities ORDER BY id`,
		"user_entities",
		fromRoot,
		toRoot,
		&result.UserEntitiesTotal,
		&result.UserEntitiesUpdated,
		&result.Samples,
	)
	if err != nil {
		return nil, err
	}

	lstUpdates, err := collectParentDirUpdates(
		db,
		`SELECT id, lst_id AS owner_id, parent_dir FROM lst_entities ORDER BY id`,
		"lst_entities",
		fromRoot,
		toRoot,
		&result.LstEntitiesTotal,
		&result.LstEntitiesUpdated,
		&result.Samples,
	)
	if err != nil {
		return nil, err
	}

	if opts.DryRun || (len(userUpdates) == 0 && len(lstUpdates) == 0) {
		return result, nil
	}

	backupPath, err := backupSQLiteFiles(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to back up database before parent_dir migration: %w", err)
	}
	result.BackupPath = backupPath

	tx, err := db.Beginx()
	if err != nil {
		return nil, fmt.Errorf("failed to begin parent_dir migration transaction: %w", err)
	}
	defer tx.Rollback()

	if err := applyParentDirUpdates(tx, "user_entities", userUpdates); err != nil {
		return nil, err
	}
	if err := applyParentDirUpdates(tx, "lst_entities", lstUpdates); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit parent_dir migration: %w", err)
	}

	if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return nil, fmt.Errorf("failed to checkpoint database after migration: %w", err)
	}

	return result, nil
}

type parentDirUpdate struct {
	ID        int64
	ParentDir string
}

func collectParentDirUpdates(
	db *sqlx.DB,
	query string,
	table string,
	fromRoot portablePath,
	toRoot portablePath,
	totalCounter *int,
	updatedCounter *int,
	samples *[]ParentDirMigrationSample,
) ([]parentDirUpdate, error) {
	rows := []parentDirOwnerRow{}
	if err := db.Select(&rows, query); err != nil {
		return nil, fmt.Errorf("failed to read %s parent_dir values: %w", table, err)
	}

	*totalCounter = len(rows)
	updates := make([]parentDirUpdate, 0, len(rows))
	desiredKeys := make(map[string]int64, len(rows))

	for _, row := range rows {
		targetParent := row.ParentDir
		rewritten, changed, err := remapParentDir(row.ParentDir, fromRoot, toRoot)
		if err != nil {
			return nil, fmt.Errorf("failed to remap %s id=%d parent_dir %q: %w", table, row.ID, row.ParentDir, err)
		}
		if changed {
			targetParent = rewritten
			updates = append(updates, parentDirUpdate{
				ID:        row.ID,
				ParentDir: targetParent,
			})
			(*updatedCounter)++
			if len(*samples) < 10 {
				*samples = append(*samples, ParentDirMigrationSample{
					Table: table,
					ID:    row.ID,
					From:  row.ParentDir,
					To:    targetParent,
				})
			}
		}

		key := fmt.Sprintf("%d|%s", row.OwnerID, strings.ToLower(targetParent))
		if existingID, ok := desiredKeys[key]; ok && existingID != row.ID {
			return nil, fmt.Errorf(
				"%s migration would create duplicate owner/path pair: owner=%d existing_id=%d conflicting_id=%d parent_dir=%q",
				table,
				row.OwnerID,
				existingID,
				row.ID,
				targetParent,
			)
		}
		desiredKeys[key] = row.ID
	}

	return updates, nil
}

func applyParentDirUpdates(tx *sqlx.Tx, table string, updates []parentDirUpdate) error {
	stmt := fmt.Sprintf("UPDATE %s SET parent_dir=? WHERE id=?", table)
	for _, update := range updates {
		if _, err := tx.Exec(stmt, update.ParentDir, update.ID); err != nil {
			return fmt.Errorf("failed to update %s id=%d parent_dir to %q: %w", table, update.ID, update.ParentDir, err)
		}
	}
	return nil
}

func remapParentDir(parentDir string, fromRoot portablePath, toRoot portablePath) (string, bool, error) {
	current, err := parsePortablePath(parentDir)
	if err != nil {
		return "", false, err
	}
	if !current.absolute {
		return parentDir, false, nil
	}

	remainder, ok := trimPortablePathPrefix(current, fromRoot)
	if !ok {
		return parentDir, false, nil
	}

	return buildPortablePath(toRoot, remainder), true, nil
}

func trimPortablePathPrefix(candidate portablePath, root portablePath) ([]string, bool) {
	if candidate.style != root.style || !candidate.absolute || !root.absolute {
		return nil, false
	}
	if candidate.volumeID != root.volumeID {
		return nil, false
	}
	if len(candidate.segments) < len(root.segments) {
		return nil, false
	}

	for i := range root.segments {
		if !portablePathSegmentEqual(candidate.segments[i], root.segments[i], candidate.style) {
			return nil, false
		}
	}

	return append([]string(nil), candidate.segments[len(root.segments):]...), true
}

func portablePathSegmentEqual(a string, b string, style portablePathStyle) bool {
	if style == portablePathStyleWindows {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func parsePortablePath(raw string) (portablePath, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return portablePath{}, fmt.Errorf("path is empty")
	}

	if strings.HasPrefix(s, `\\`) || strings.HasPrefix(s, `//`) {
		return parseWindowsUNCPath(s)
	}
	if len(s) >= 2 && s[1] == ':' && isASCIIAlpha(s[0]) {
		return parseWindowsDrivePath(s), nil
	}
	if strings.Contains(s, `\`) {
		return parseWindowsRelativePath(s), nil
	}
	if strings.HasPrefix(s, "/") {
		return parsePosixPath(s), nil
	}
	return parsePosixRelativePath(s), nil
}

func parseWindowsDrivePath(raw string) portablePath {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), `\`, "/")
	drive := strings.ToLower(normalized[:2])
	absolute := len(normalized) >= 3 && normalized[2] == '/'

	remainder := ""
	if len(normalized) > 2 {
		remainder = normalized[2:]
	}

	return portablePath{
		style:      portablePathStyleWindows,
		absolute:   absolute,
		volumeID:   drive,
		rootPrefix: strings.ToUpper(normalized[:1]) + ":",
		segments:   cleanPortableSegments(remainder, true),
	}
}

func parseWindowsUNCPath(raw string) (portablePath, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), `\`, "/")
	normalized = strings.TrimPrefix(strings.TrimPrefix(normalized, "//"), "/")
	parts := splitPortableSegments(normalized)
	if len(parts) < 2 {
		return portablePath{}, fmt.Errorf("UNC path %q is missing host/share components", raw)
	}

	host := parts[0]
	share := parts[1]
	return portablePath{
		style:      portablePathStyleWindows,
		absolute:   true,
		volumeID:   strings.ToLower(host + "/" + share),
		rootPrefix: `\\` + host + `\` + share,
		segments:   cleanSegments(parts[2:]),
	}, nil
}

func parseWindowsRelativePath(raw string) portablePath {
	return portablePath{
		style:    portablePathStyleWindows,
		absolute: false,
		segments: cleanPortableSegments(strings.ReplaceAll(raw, `\`, "/"), false),
	}
}

func parsePosixPath(raw string) portablePath {
	return portablePath{
		style:      portablePathStylePosix,
		absolute:   true,
		volumeID:   "/",
		rootPrefix: "/",
		segments:   cleanPortableSegments(raw, true),
	}
}

func parsePosixRelativePath(raw string) portablePath {
	return portablePath{
		style:    portablePathStylePosix,
		absolute: false,
		segments: cleanPortableSegments(raw, false),
	}
}

func cleanPortableSegments(raw string, absolute bool) []string {
	cleaned := path.Clean(strings.ReplaceAll(raw, `\`, "/"))
	if absolute {
		cleaned = strings.TrimPrefix(cleaned, "/")
	}
	if cleaned == "." || cleaned == "" {
		return nil
	}
	return splitPortableSegments(cleaned)
}

func splitPortableSegments(raw string) []string {
	return cleanSegments(strings.Split(raw, "/"))
}

func cleanSegments(parts []string) []string {
	segs := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		segs = append(segs, part)
	}
	return segs
}

func buildPortablePath(root portablePath, remainder []string) string {
	allSegments := append(append([]string(nil), root.segments...), remainder...)

	switch root.style {
	case portablePathStyleWindows:
		if strings.HasPrefix(root.rootPrefix, `\\`) {
			if len(allSegments) == 0 {
				return root.rootPrefix
			}
			return root.rootPrefix + `\` + strings.Join(allSegments, `\`)
		}
		if len(allSegments) == 0 {
			return root.rootPrefix + `\`
		}
		return root.rootPrefix + `\` + strings.Join(allSegments, `\`)
	default:
		if len(allSegments) == 0 {
			return "/"
		}
		return "/" + strings.Join(allSegments, "/")
	}
}

func isASCIIAlpha(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}
