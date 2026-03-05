package agent

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
	mu sync.Mutex
}

func OpenSQLiteStore(workspace string) (*SQLiteStore, error) {
	if stringsTrimLower(os.Getenv("NIBOT_STORAGE")) != "sqlite" && stringsTrimLower(os.Getenv("NIBOT_MEMORY_DB")) != "sqlite" {
		return nil, nil
	}
	p := filepath.Join(workspace, "data", "nibot.db")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", p)
	if err != nil {
		return nil, err
	}
	s := &SQLiteStore{db: db}
	if err := s.ensureSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) Close() {
	if s == nil || s.db == nil {
		return
	}
	_ = s.db.Close()
}

func (s *SQLiteStore) ensureSchema() error {
	if s == nil || s.db == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmts := []string{
		`create table if not exists sessions (
			session_id text primary key,
			start_time text,
			last_activity text,
			message_count integer,
			tool_calls integer,
			approvals integer,
			denials integer,
			current_task text
		);`,
		`create table if not exists messages (
			id integer primary key autoincrement,
			session_id text,
			role text,
			content text,
			created_at text
		);`,
		`create table if not exists tool_audits (
			id integer primary key autoincrement,
			session_id text,
			tool text,
			args text,
			ok integer,
			error text,
			output text,
			created_at text
		);`,
		`create table if not exists memories (
			id integer primary key autoincrement,
			scope text,
			tags text,
			content text,
			created_at text,
			fingerprint text,
			source text,
			updated_at text
		);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	if err := s.ensureMemoryColumnsLocked(); err != nil {
		return err
	}
	if err := s.ensureMemoryIndexesLocked(); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) ensureMemoryColumnsLocked() error {
	cols, err := s.tableColumnsLocked("memories")
	if err != nil {
		return err
	}
	need := map[string]string{
		"fingerprint": `alter table memories add column fingerprint text;`,
		"source":      `alter table memories add column source text;`,
		"updated_at":  `alter table memories add column updated_at text;`,
	}
	for col, ddl := range need {
		if cols[col] {
			continue
		}
		if _, err := s.db.Exec(ddl); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) ensureMemoryIndexesLocked() error {
	stmts := []string{
		`create index if not exists idx_memories_scope_id on memories(scope, id);`,
		`create index if not exists idx_memories_scope_created_at on memories(scope, created_at);`,
		`create unique index if not exists ux_memories_scope_fingerprint on memories(scope, fingerprint) where fingerprint is not null and fingerprint <> '';`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) tableColumnsLocked(table string) (map[string]bool, error) {
	out := map[string]bool{}
	rows, err := s.db.Query(`select name from pragma_table_info(?)`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[strings.ToLower(strings.TrimSpace(name))] = true
	}
	return out, nil
}

func (s *SQLiteStore) UpsertSession(state *SessionState) error {
	if s == nil || s.db == nil || state == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(
		`insert into sessions(session_id,start_time,last_activity,message_count,tool_calls,approvals,denials,current_task)
		 values(?,?,?,?,?,?,?,?)
		 on conflict(session_id) do update set
		 start_time=excluded.start_time,
		 last_activity=excluded.last_activity,
		 message_count=excluded.message_count,
		 tool_calls=excluded.tool_calls,
		 approvals=excluded.approvals,
		 denials=excluded.denials,
		 current_task=excluded.current_task`,
		state.SessionID,
		state.StartTime.Format(time.RFC3339Nano),
		state.LastActivity.Format(time.RFC3339Nano),
		state.MessageCount,
		state.ToolCalls,
		state.Approvals,
		state.Denials,
		state.CurrentTask,
	)
	return err
}

func (s *SQLiteStore) InsertMessage(sessionID, role, content string) error {
	if s == nil || s.db == nil {
		return nil
	}
	sessionID = stringsTrimSpace(sessionID)
	role = stringsTrimSpace(role)
	if sessionID == "" || role == "" {
		return fmt.Errorf("invalid message: empty sessionID or role")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(
		`insert into messages(session_id,role,content,created_at) values(?,?,?,?)`,
		sessionID,
		role,
		content,
		time.Now().Format(time.RFC3339Nano),
	)
	return err
}

func (s *SQLiteStore) InsertToolAudit(sessionID string, calls []ExecCall, results []ToolResult) error {
	if s == nil || s.db == nil {
		return nil
	}
	sessionID = stringsTrimSpace(sessionID)
	if sessionID == "" {
		return fmt.Errorf("invalid tool audit: empty sessionID")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Format(time.RFC3339Nano)
	for i, call := range calls {
		var r ToolResult
		if i < len(results) {
			r = results[i]
		} else {
			r = ToolResult{Tool: call.Tool, OK: false, Error: "missing result"}
		}
		ok := 0
		if r.OK {
			ok = 1
		}
		_, err := s.db.Exec(
			`insert into tool_audits(session_id,tool,args,ok,error,output,created_at) values(?,?,?,?,?,?,?)`,
			sessionID,
			call.Tool,
			call.ArgsRaw,
			ok,
			r.Error,
			r.Output,
			now,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

type MemoryItem struct {
	ID        int64
	Scope     string
	Tags      string
	Content   string
	CreatedAt time.Time
	Fingerprint string
	Source      string
	UpdatedAt   time.Time
}

func (s *SQLiteStore) InsertMemory(scope, tags, content string) (int64, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("sqlite store not enabled")
	}
	id, _, err := s.UpsertMemory(scope, tags, content, "")
	return id, err
}

func (s *SQLiteStore) UpsertMemory(scope, tags, content, source string) (id int64, action string, err error) {
	if s == nil || s.db == nil {
		return 0, "", fmt.Errorf("sqlite store not enabled")
	}
	scope = stringsTrimSpace(scope)
	tags = stringsTrimSpace(tags)
	content = stringsTrimSpace(content)
	source = stringsTrimSpace(source)
	if scope == "" {
		scope = "global"
	}
	if content == "" {
		return 0, "", fmt.Errorf("empty content")
	}

	fp := memoryFingerprint(content)
	now := time.Now().Format(time.RFC3339Nano)

	s.mu.Lock()
	defer s.mu.Unlock()

	var existingID int64
	var existingTags string
	var existingContent string
	row := s.db.QueryRow(`select id,tags,content from memories where scope = ? and fingerprint = ? order by id desc limit 1`, scope, fp)
	switch err := row.Scan(&existingID, &existingTags, &existingContent); err {
	case nil:
		mergedTags := mergeTags(existingTags, tags)
		if stringsTrimSpace(existingContent) == content && stringsTrimSpace(existingTags) == mergedTags && source == "" {
			return existingID, "unchanged", nil
		}
		if source == "" {
			_, err = s.db.Exec(`update memories set tags=?, content=?, updated_at=? where id=?`, mergedTags, content, now, existingID)
		} else {
			_, err = s.db.Exec(`update memories set tags=?, content=?, source=?, updated_at=? where id=?`, mergedTags, content, source, now, existingID)
		}
		if err != nil {
			return 0, "", err
		}
		return existingID, "updated", nil
	case sql.ErrNoRows:
	default:
		return 0, "", err
	}

	row = s.db.QueryRow(`select id,tags from memories where scope = ? and content = ? order by id desc limit 1`, scope, content)
	switch err := row.Scan(&existingID, &existingTags); err {
	case nil:
		mergedTags := mergeTags(existingTags, tags)
		if source == "" {
			_, err = s.db.Exec(`update memories set tags=?, fingerprint=?, updated_at=? where id=?`, mergedTags, fp, now, existingID)
		} else {
			_, err = s.db.Exec(`update memories set tags=?, fingerprint=?, source=?, updated_at=? where id=?`, mergedTags, fp, source, now, existingID)
		}
		if err != nil {
			return 0, "", err
		}
		return existingID, "updated", nil
	case sql.ErrNoRows:
	default:
		return 0, "", err
	}

	res, err := s.db.Exec(
		`insert into memories(scope,tags,content,created_at,fingerprint,source,updated_at) values(?,?,?,?,?,?,?)`,
		scope,
		tags,
		content,
		now,
		fp,
		source,
		now,
	)
	if err != nil {
		return 0, "", err
	}
	newID, _ := res.LastInsertId()
	return newID, "inserted", nil
}

func (s *SQLiteStore) DeleteMemory(id int64) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("sqlite store not enabled")
	}
	if id <= 0 {
		return fmt.Errorf("invalid id")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`delete from memories where id = ?`, id)
	return err
}

func (s *SQLiteStore) ListMemories(scope string, limit int) ([]MemoryItem, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("sqlite store not enabled")
	}
	scope = stringsTrimSpace(scope)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var rows *sql.Rows
	var err error
	if scope == "" || stringsTrimLower(scope) == "all" {
		rows, err = s.db.Query(`select id,scope,tags,content,created_at,fingerprint,source,updated_at from memories order by id desc limit ?`, limit)
	} else {
		rows, err = s.db.Query(`select id,scope,tags,content,created_at,fingerprint,source,updated_at from memories where scope = ? order by id desc limit ?`, scope, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MemoryItem
	for rows.Next() {
		var it MemoryItem
		var created string
		var updated sql.NullString
		if err := rows.Scan(&it.ID, &it.Scope, &it.Tags, &it.Content, &created, &it.Fingerprint, &it.Source, &updated); err != nil {
			return nil, err
		}
		if t, err := time.Parse(time.RFC3339Nano, created); err == nil {
			it.CreatedAt = t
		}
		if updated.Valid {
			if t, err := time.Parse(time.RFC3339Nano, updated.String); err == nil {
				it.UpdatedAt = t
			}
		}
		out = append(out, it)
	}
	return out, nil
}

func (s *SQLiteStore) SearchMemories(scope, query string, limit int) ([]MemoryItem, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("sqlite store not enabled")
	}
	scope = stringsTrimSpace(scope)
	query = stringsTrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	pat := "%" + query + "%"
	var rows *sql.Rows
	var err error
	if scope == "" || stringsTrimLower(scope) == "all" {
		rows, err = s.db.Query(`select id,scope,tags,content,created_at,fingerprint,source,updated_at from memories where content like ? order by id desc limit ?`, pat, limit)
	} else {
		rows, err = s.db.Query(`select id,scope,tags,content,created_at,fingerprint,source,updated_at from memories where scope = ? and content like ? order by id desc limit ?`, scope, pat, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MemoryItem
	for rows.Next() {
		var it MemoryItem
		var created string
		var updated sql.NullString
		if err := rows.Scan(&it.ID, &it.Scope, &it.Tags, &it.Content, &created, &it.Fingerprint, &it.Source, &updated); err != nil {
			return nil, err
		}
		if t, err := time.Parse(time.RFC3339Nano, created); err == nil {
			it.CreatedAt = t
		}
		if updated.Valid {
			if t, err := time.Parse(time.RFC3339Nano, updated.String); err == nil {
				it.UpdatedAt = t
			}
		}
		out = append(out, it)
	}
	return out, nil
}

func (s *SQLiteStore) MemoryStats() (count int64, err error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("sqlite store not enabled")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRow(`select count(1) from memories`)
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func stringsTrimLower(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func stringsTrimSpace(v string) string {
	return strings.TrimSpace(v)
}

func memoryFingerprint(content string) string {
	n := strings.ToLower(strings.TrimSpace(content))
	n = strings.Join(strings.Fields(n), " ")
	sum := sha256.Sum256([]byte(n))
	return hex.EncodeToString(sum[:])
}

func mergeTags(existing, incoming string) string {
	existing = strings.TrimSpace(existing)
	incoming = strings.TrimSpace(incoming)
	if existing == "" {
		return incoming
	}
	if incoming == "" {
		return existing
	}
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = strings.ToLower(strings.TrimSpace(s))
		s = strings.Trim(s, ",")
		if s == "" {
			return
		}
		if seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	for _, part := range strings.FieldsFunc(existing, func(r rune) bool { return r == ',' || r == ';' || r == '|' }) {
		add(part)
	}
	for _, part := range strings.FieldsFunc(incoming, func(r rune) bool { return r == ',' || r == ';' || r == '|' }) {
		add(part)
	}
	return strings.Join(out, ",")
}
