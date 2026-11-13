package main

import (
	"bufio"
	"database/sql"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"text/tabwriter"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	sql.Register("sqlite3_test", &sqlite3.SQLiteDriver{})
	var err error
	testDB, err = sql.Open("sqlite3_test", ":memory:")
	if err != nil {
		panic(err)
	}
	testDB.SetMaxOpenConns(1)
	if _, err = testDB.Exec("CREATE TABLE IF NOT EXISTS t(id INTEGER PRIMARY KEY, name TEXT, value INTEGER)"); err != nil {
		panic(err)
	}
	if _, err = testDB.Exec("CREATE TABLE IF NOT EXISTS translog(id INTEGER PRIMARY KEY AUTOINCREMENT, time timestamp, client text, query text)"); err != nil {
		panic(err)
	}
	if _, err = testDB.Exec("INSERT INTO t(name,value) VALUES('alice',1),('bob',2),(NULL,3)"); err != nil {
		panic(err)
	}
	code := m.Run()
	testDB.Close()
	os.Exit(code)
}

func newPipe() (net.Conn, net.Conn) {
	return net.Pipe()
}

func readUntil(r *bufio.Reader, delim byte, max int) string {
	var buf []byte
	for i := 0; i < max; i++ {
		b, err := r.ReadByte()
		if err != nil {
			break
		}
		buf = append(buf, b)
		if b == delim {
			break
		}
	}
	return string(buf)
}

func runAndCapture(s, c net.Conn, f func()) string {
	var sb strings.Builder
	var mu sync.Mutex
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 4096)
		for {
			n, err := c.Read(buf)
			if n > 0 {
				mu.Lock()
				sb.Write(buf[:n])
				mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
	f()
	_ = c.Close()
	<-readDone
	mu.Lock()
	defer mu.Unlock()
	return sb.String()
}

func TestReadUntil(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()

	go func() {
		_, _ = s.Write([]byte("hello\nworld\n"))
	}()

	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 1024)
	if out != "hello\n" {
		t.Errorf("expected 'hello\\n', got %q", out)
	}
}

func TestReadUntilMax(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()

	go func() {
		_, _ = s.Write([]byte("short"))
		_ = s.Close()
	}()

	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 3)
	if len(out) != 3 {
		t.Errorf("expected 3 bytes, got %d", len(out))
	}
}

func TestReadUntilEOF(t *testing.T) {
	s, c := newPipe()
	defer c.Close()

	go func() {
		_, _ = s.Write([]byte("data"))
		_ = s.Close()
	}()

	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 1024)
	if out != "data" {
		t.Errorf("expected 'data', got %q", out)
	}
}

func TestNewPipe(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	if s == nil || c == nil {
		t.Error("expected non-nil connections")
	}
}

func TestRunFlagParsingError(t *testing.T) {
	result := run([]string{"-unknown-flag"})
	if result != 1 {
		t.Errorf("expected 1, got %d", result)
	}
}

func TestRunInvalidFlag(t *testing.T) {
	result := run([]string{"--invalid"})
	if result != 1 {
		t.Errorf("expected 1 for invalid flag, got %d", result)
	}
}

func TestRunEmptyFlag(t *testing.T) {
	result := run([]string{"-"})
	if result != 1 {
		t.Errorf("expected 1 for bare dash, got %d", result)
	}
}

func TestInitDB(t *testing.T) {
	db, err := initDB()
	if err != nil {
		t.Skipf("initDB unavailable in this environment: %v", err)
	}
	defer func() { _ = db.Close() }()
	if db == nil {
		t.Fatal("expected db")
	}
}

func TestHandleDotCommandQuit(t *testing.T) {
	s, c := newPipe()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	out := runAndCapture(s, c, func() {
		done := make(chan bool, 1)
		go func() { done <- handleDotCommand(s, testDB, ".quit", w) }()
		if !<-done {
			t.Error(".quit should return true")
		}
	})
	if !strings.Contains(out, "Bye!") {
		t.Errorf("expected 'Bye!', got %q", out)
	}
}

func TestHandleDotCommandExit(t *testing.T) {
	s, c := newPipe()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	out := runAndCapture(s, c, func() {
		done := make(chan bool, 1)
		go func() { done <- handleDotCommand(s, testDB, ".exit", w) }()
		if !<-done {
			t.Error(".exit should return true")
		}
	})
	if !strings.Contains(out, "Bye!") {
		t.Errorf("expected 'Bye!', got %q", out)
	}
}

func TestHandleDotCommandTables(t *testing.T) {
	s, c := newPipe()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	out := runAndCapture(s, c, func() {
		done := make(chan bool, 1)
		go func() { done <- handleDotCommand(s, testDB, ".tables", w) }()
		if <-done {
			t.Error(".tables should return false")
		}
	})
	if !strings.Contains(out, "t") {
		t.Errorf("expected table 't' in output, got %q", out)
	}
}

func TestHandleDotCommandSchema(t *testing.T) {
	s, c := newPipe()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	out := runAndCapture(s, c, func() {
		done := make(chan bool, 1)
		go func() { done <- handleDotCommand(s, testDB, ".schema", w) }()
		if <-done {
			t.Error(".schema should return false")
		}
	})
	if !strings.Contains(out, "CREATE TABLE") {
		t.Errorf("expected CREATE TABLE in output, got %q", out)
	}
}

func TestHandleDotCommandDatabases(t *testing.T) {
	s, c := newPipe()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	out := runAndCapture(s, c, func() {
		done := make(chan bool, 1)
		go func() { done <- handleDotCommand(s, testDB, ".databases", w) }()
		if <-done {
			t.Error(".databases should return false")
		}
	})
	if !strings.Contains(out, "main") {
		t.Errorf("expected 'main' in output, got %q", out)
	}
}

func TestHandleDotCommandHelp(t *testing.T) {
	s, c := newPipe()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	out := runAndCapture(s, c, func() {
		done := make(chan bool, 1)
		go func() { done <- handleDotCommand(s, testDB, ".help", w) }()
		if <-done {
			t.Error(".help should return false")
		}
	})
	if !strings.Contains(out, ".tables") || !strings.Contains(out, ".quit") {
		t.Errorf("expected help text, got %q", out)
	}
}

func TestHandleDotCommandUnknown(t *testing.T) {
	s, c := newPipe()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	out := runAndCapture(s, c, func() {
		done := make(chan bool, 1)
		go func() { done <- handleDotCommand(s, testDB, ".nonsense", w) }()
		if <-done {
			t.Error("unknown cmd should return false")
		}
	})
	if !strings.Contains(out, "Unknown command") {
		t.Errorf("expected 'Unknown command', got %q", out)
	}
}

func TestHandleSQLCommandSelect(t *testing.T) {
	s, c := newPipe()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	out := runAndCapture(s, c, func() {
		handleSQLCommand(s, testDB, "SELECT name FROM t WHERE id=1", w)
	})
	if !strings.Contains(out, "alice") {
		t.Errorf("expected 'alice', got %q", out)
	}
}

func TestHandleSQLCommandPragma(t *testing.T) {
	s, c := newPipe()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	out := runAndCapture(s, c, func() {
		handleSQLCommand(s, testDB, "PRAGMA database_list", w)
	})
	if !strings.Contains(out, "main") {
		t.Errorf("expected 'main', got %q", out)
	}
}

func TestHandleSQLCommandInsert(t *testing.T) {
	s, c := newPipe()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	out := runAndCapture(s, c, func() {
		handleSQLCommand(s, testDB, "INSERT INTO t(name,value) VALUES('x',10)", w)
	})
	if !strings.Contains(out, "OK") || !strings.Contains(out, "1 rows") {
		t.Errorf("expected 'OK. 1 rows affected.', got %q", out)
	}
}

func TestHandleSQLCommandExecError(t *testing.T) {
	s, c := newPipe()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	out := runAndCapture(s, c, func() {
		handleSQLCommand(s, testDB, "INSERT INTO nonexistent(col) VALUES(1)", w)
	})
	if !strings.Contains(out, "Error") {
		t.Errorf("expected 'Error', got %q", out)
	}
}

func TestRunQuerySuccess(t *testing.T) {
	s, c := newPipe()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	out := runAndCapture(s, c, func() {
		runQuery(s, testDB, "SELECT name,value FROM t ORDER BY id", w)
	})
	if !strings.Contains(out, "alice") || !strings.Contains(out, "bob") {
		t.Errorf("expected 'alice' and 'bob', got %q", out)
	}
}

func TestRunQueryEmpty(t *testing.T) {
	s, c := newPipe()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	out := runAndCapture(s, c, func() {
		runQuery(s, testDB, "SELECT name FROM t WHERE id=999", w)
	})
	if strings.Contains(out, "alice") {
		t.Errorf("expected no data rows, got %q", out)
	}
}

func TestRunQueryNullValues(t *testing.T) {
	s, c := newPipe()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	out := runAndCapture(s, c, func() {
		runQuery(s, testDB, "SELECT name,value FROM t WHERE id=3", w)
	})
	if !strings.Contains(out, "NULL") {
		t.Errorf("expected NULL in output, got %q", out)
	}
}

func TestRunQueryError(t *testing.T) {
	s, c := newPipe()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	out := runAndCapture(s, c, func() {
		runQuery(s, testDB, "SELECT * FROM nonexistent", w)
	})
	if !strings.Contains(out, "Error") {
		t.Errorf("expected 'Error', got %q", out)
	}
}

func TestHandleConnectionFullSession(t *testing.T) {
	s, c := newPipe()
	defer c.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleConnection(s, testDB, "testhost")
	}()

	// read welcome
	buf := make([]byte, 4096)
	_, _ = c.Read(buf)

	wc := bufio.NewWriter(c)
	_, _ = wc.WriteString("\n")
	_, _ = wc.WriteString("SELECT name FROM t WHERE id=1\n")
	_, _ = wc.WriteString(".quit\n")
	_ = wc.Flush()

	c.Close()
	wg.Wait()
}

func TestAcceptLoopShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	stop := make(chan os.Signal, 1)
	done := make(chan struct{})
	go func() {
		acceptLoop(listener, testDB, "testhost", stop)
		close(done)
	}()

	_ = listener.Close()
	stop <- os.Interrupt
	<-done
}

func readPrompt(r *bufio.Reader) {
	var last string
	for {
		b, err := r.ReadByte()
		if err != nil {
			return
		}
		last += string(b)
		if len(last) > 2 {
			last = last[len(last)-2:]
		}
		if last == "> " {
			return
		}
	}
}

func TestAcceptLoopConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	stop := make(chan os.Signal, 1)
	go acceptLoop(listener, testDB, "testhost", stop)

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	reader := bufio.NewReader(conn)
	readPrompt(reader)

	writer := bufio.NewWriter(conn)
	_, _ = writer.WriteString(".quit\n")
	_ = writer.Flush()
	_ = conn.Close()

	_ = listener.Close()
	stop <- os.Interrupt
}

func TestHandleConnectionConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, c := newPipe()
			defer func() { _ = c.Close() }()

			var innerWg sync.WaitGroup
			innerWg.Add(1)
			go func() {
				defer innerWg.Done()
				handleConnection(s, testDB, "testhost")
			}()

			// wait for prompt
			r := bufio.NewReader(c)
			readPrompt(r)

			wc := bufio.NewWriter(c)
			_, _ = wc.WriteString("SELECT 1\n")
			_, _ = wc.WriteString(".quit\n")
			_ = wc.Flush()
			_ = c.Close()
			innerWg.Wait()
		}()
	}
	wg.Wait()
}

// Fuzz tests

func FuzzReadUntil(f *testing.F) {
	f.Add("hello\nworld\n", 5)
	f.Add("", 0)
	f.Fuzz(func(t *testing.T, input string, max int) {
		if max < 0 {
			max = 0
		}
		if max > 1024 {
			max = 1024
		}
		s, c := newPipe()
		defer func() { _ = s.Close() }()
		defer func() { _ = c.Close() }()
		go func() {
			_, _ = s.Write([]byte(input))
			_ = s.Close()
		}()
		r := bufio.NewReader(c)
		_ = readUntil(r, '\n', max)
	})
}

func FuzzHandleDotCommand(f *testing.F) {
	f.Add(".help")
	f.Add(".tables")
	f.Add(".quit")
	f.Fuzz(func(t *testing.T, input string) {
		db := newFuzzDB(t)
		defer func() { _ = db.Close() }()
		s, c := newPipe()
		w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
		done := make(chan bool, 1)
		go func() { done <- handleDotCommand(s, db, input, w) }()
		go func() {
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			_ = s.Close()
			_ = c.Close()
		}()
		// drain
		buf := make([]byte, 4096)
		for {
			_, err := c.Read(buf)
			if err != nil {
				break
			}
		}
	})
}

// Suppress unused import
var _ = io.EOF
var _ = time.Second

// Benchmarks

func BenchmarkReadUntil(b *testing.B) {
	for i := 0; i < b.N; i++ {
		s, c := newPipe()
		go func() {
			_, _ = s.Write([]byte("hello\nworld\n"))
			_ = s.Close()
		}()
		r := bufio.NewReader(c)
		_ = readUntil(r, '\n', 1024)
		_ = c.Close()
	}
}

func BenchmarkHandleDotCommand(b *testing.B) {
	for i := 0; i < b.N; i++ {
		s, c := newPipe()
		w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
		done := make(chan bool, 1)
		go func() {
			done <- handleDotCommand(s, testDB, ".tables", w)
			_ = s.Close()
		}()
		buf := make([]byte, 4096)
		for {
			_, err := c.Read(buf)
			if err != nil {
				break
			}
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
		_ = c.Close()
	}
}

// Accuracy / Oracle gates

func TestAccuracyGate(t *testing.T) {
	s, c := newPipe()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	out := runAndCapture(s, c, func() {
		runQuery(s, testDB, "SELECT name FROM t WHERE id = 1", w)
	})
	if !strings.Contains(out, "alice") {
		t.Fatalf("accuracy gate failed: %q", out)
	}
}

func TestOracleParity(t *testing.T) {
	s, c := newPipe()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	out := runAndCapture(s, c, func() {
		runQuery(s, testDB, "SELECT COUNT(*) FROM t WHERE name = 'alice'", w)
	})
	if !strings.Contains(out, "1") {
		t.Fatalf("oracle parity failed: %q", out)
	}
}

// Additional fuzz targets

func newFuzzDB(t testing.TB) *sql.DB {
	db, err := sql.Open("sqlite3_test", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("CREATE TABLE t(id INTEGER PRIMARY KEY, name TEXT, value INTEGER)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO t(name,value) VALUES('alice',1),('bob',2),(NULL,3)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE translog(id INTEGER PRIMARY KEY AUTOINCREMENT, time timestamp, client text, query text)"); err != nil {
		t.Fatal(err)
	}
	return db
}

func FuzzHandleSQLCommand(f *testing.F) {
	f.Add("SELECT 1")
	f.Add("PRAGMA database_list")
	f.Fuzz(func(t *testing.T, input string) {
		db := newFuzzDB(t)
		defer func() { _ = db.Close() }()
		s, c := newPipe()
		w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
		go func() {
			handleSQLCommand(s, db, input, w)
			_ = s.Close()
		}()
		buf := make([]byte, 4096)
		for {
			_, err := c.Read(buf)
			if err != nil {
				break
			}
		}
		_ = c.Close()
	})
}

func FuzzRunQuery(f *testing.F) {
	f.Add("SELECT name FROM t WHERE id = 1")
	f.Fuzz(func(t *testing.T, input string) {
		db := newFuzzDB(t)
		defer func() { _ = db.Close() }()
		s, c := newPipe()
		w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
		go func() {
			runQuery(s, db, input, w)
			_ = s.Close()
		}()
		buf := make([]byte, 4096)
		for {
			_, err := c.Read(buf)
			if err != nil {
				break
			}
		}
		_ = c.Close()
	})
}

func TestRunQueryRowLimit(t *testing.T) {
	db, err := sql.Open("sqlite3_test", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("CREATE TABLE big(id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1005; i++ {
		if _, err := tx.Exec("INSERT INTO big(id) VALUES(?)", i+1); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	s, c := newPipe()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	out := runAndCapture(s, c, func() {
		runQuery(s, db, "SELECT id FROM big", w)
	})
	if !strings.Contains(out, "truncated") {
		t.Fatalf("expected truncation, got %q", out[:100])
	}
}
