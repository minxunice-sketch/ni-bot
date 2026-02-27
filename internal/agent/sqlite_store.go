package agent

import (
	"database/sql"
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
	if stringsTrimLower(os.Getenv("NIBOT_STORAGE")) != "sqlite" {
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
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
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

func stringsTrimLower(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func stringsTrimSpace(v string) string {
	return strings.TrimSpace(v)
}
