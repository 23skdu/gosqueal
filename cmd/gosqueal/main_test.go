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
	if _, err = testDB.Exec("CREATE TABLE IF NOT EXISTS t(id INTEGER PRIMARY KEY, name TEXT, value INTEGER)"); err != nil {
		panic(err)
	}
	if _, err = testDB.Exec("INSERT INTO t(name,value) VALUES('alice',1),('bob',2),(NULL,3)"); err != nil {
		panic(err)
	}
	code := m.Run()
	testDB.Close()
	os.Exit(code)
}

func readUntil(r *bufio.Reader, delim byte, max int) string {
	buf := make([]byte, max)
	for i := 0; i < max; i++ {
		b, err := r.ReadByte()
		if err != nil {
			break
		}
		buf[i] = b
		if b == delim {
			break
		}
	}
	return string(buf)
}

func TestHandleDotCommandQuit(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan bool, 1)
	go func() {
		done <- handleDotCommand(s, testDB, ".quit", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 1024)
	if !<-done {
		t.Error(".quit should return true")
	}
	if !strings.Contains(out, "Bye!") {
		t.Errorf("expected 'Bye!', got %q", out)
	}
}

func TestHandleDotCommandExit(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan bool, 1)
	go func() {
		done <- handleDotCommand(s, testDB, ".exit", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 1024)
	if !<-done {
		t.Error(".exit should return true")
	}
	if !strings.Contains(out, "Bye!") {
		t.Errorf("expected 'Bye!', got %q", out)
	}
}

func TestHandleDotCommandTables(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan bool, 1)
	go func() {
		done <- handleDotCommand(s, testDB, ".tables", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	if <-done {
		t.Error(".tables should return false")
	}
	if !strings.Contains(out, "t") {
		t.Errorf("expected table 't' in output, got %q", out)
	}
}

func TestHandleDotCommandSchema(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan bool, 1)
	go func() {
		done <- handleDotCommand(s, testDB, ".schema", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil || strings.TrimSpace(line) == "" {
			break
		}
	}
	if <-done {
		t.Error(".schema should return false")
	}
	if !strings.Contains(out.String(), "CREATE TABLE") {
		t.Errorf("expected CREATE TABLE in output, got %q", out.String())
	}
}

func TestHandleDotCommandDatabases(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan bool, 1)
	go func() {
		done <- handleDotCommand(s, testDB, ".databases", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	if <-done {
		t.Error(".databases should return false")
	}
	if !strings.Contains(out, "main") {
		t.Errorf("expected 'main' in output, got %q", out)
	}
}

func TestHandleDotCommandHelp(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan bool, 1)
	go func() {
		done <- handleDotCommand(s, testDB, ".help", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for i := 0; i < 4; i++ {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
	}
	if <-done {
		t.Error(".help should return false")
	}
	sout := out.String()
	if !strings.Contains(sout, ".tables") || !strings.Contains(sout, ".quit") {
		t.Errorf("expected help text, got %q", sout)
	}
}

func TestHandleDotCommandUnknown(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan bool, 1)
	go func() {
		done <- handleDotCommand(s, testDB, ".nonsense", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 1024)
	if <-done {
		t.Error("unknown cmd should return false")
	}
	if !strings.Contains(out, "Unknown command") {
		t.Errorf("expected 'Unknown command', got %q", out)
	}
}

func TestHandleSQLCommandSelect(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		handleSQLCommand(s, testDB, "SELECT name FROM t WHERE id=1", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "alice") {
		t.Errorf("expected 'alice', got %q", out)
	}
}

func TestHandleSQLCommandPragma(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		handleSQLCommand(s, testDB, "PRAGMA database_list", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "main") {
		t.Errorf("expected 'main', got %q", out)
	}
}

func TestHandleSQLCommandInsert(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		handleSQLCommand(s, testDB, "INSERT INTO t(name,value) VALUES('x',10)", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 1024)
	<-done
	if !strings.Contains(out, "OK") || !strings.Contains(out, "1 rows") {
		t.Errorf("expected 'OK. 1 rows affected.', got %q", out)
	}
}

func TestHandleSQLCommandExecError(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		handleSQLCommand(s, testDB, "INSERT INTO nonexistent(col) VALUES(1)", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 1024)
	<-done
	if !strings.Contains(out, "Error") {
		t.Errorf("expected 'Error', got %q", out)
	}
}

func TestRunQuerySuccess(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT name,value FROM t ORDER BY id", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if !strings.Contains(out.String(), "alice") || !strings.Contains(out.String(), "bob") {
		t.Errorf("expected 'alice' and 'bob', got %q", out.String())
	}
}

func TestRunQueryEmpty(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT name FROM t WHERE id=999", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if strings.Contains(out, "alice") {
		t.Errorf("expected no data rows, got %q", out)
	}
}

func TestRunQueryNullValues(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT name,value FROM t WHERE id=3", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if !strings.Contains(out.String(), "NULL") {
		t.Errorf("expected NULL in output, got %q", out.String())
	}
}

func TestRunQueryError(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT * FROM nonexistent", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 1024)
	<-done
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

	welcome := waitForPrompt(c)
	if !strings.Contains(welcome, "Connected to gosqueal") {
		t.Errorf("expected welcome message, got %q", welcome)
	}

	wc := bufio.NewWriter(c)
	wc.WriteString("\n")
	wc.WriteString("SELECT name FROM t WHERE id=1\n")
	wc.WriteString(".quit\n")
	wc.Flush()

	c.Close()
	wg.Wait()
}

func TestHandleConnectionDisconnect(t *testing.T) {
	s, c := newPipe()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleConnection(s, testDB, "testhost")
	}()

	waitForPrompt(c)

	wc := bufio.NewWriter(c)
	wc.WriteString("SELECT 1\n")
	wc.Flush()

	c.Close()
	wg.Wait()
}

func TestHandleConnectionSQLInsert(t *testing.T) {
	s, c := newPipe()
	defer c.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleConnection(s, testDB, "testhost")
	}()

	waitForPrompt(c)

	wc := bufio.NewWriter(c)
	wc.WriteString("INSERT INTO t(name,value) VALUES('test',99)\n")
	wc.WriteString(".quit\n")
	wc.Flush()

	c.Close()
	wg.Wait()
}

func TestHandleConnectionPragma(t *testing.T) {
	s, c := newPipe()
	defer c.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleConnection(s, testDB, "testhost")
	}()

	waitForPrompt(c)

	wc := bufio.NewWriter(c)
	wc.WriteString("PRAGMA database_list\n")
	wc.WriteString(".quit\n")
	wc.Flush()

	c.Close()
	wg.Wait()
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

func TestRunQueryMultipleRows(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT id,name FROM t ORDER BY id", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	sout := out.String()
	if !strings.Contains(sout, "alice") || !strings.Contains(sout, "bob") || !strings.Contains(sout, "NULL") {
		t.Errorf("expected all values, got %q", sout)
	}
}

func TestNewPipe(t *testing.T) {
	s, c := net.Pipe()
	defer s.Close()
	defer c.Close()
	if s == nil || c == nil {
		t.Error("expected non-nil connections")
	}
}

func TestReadUntil(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()

	go func() {
		s.Write([]byte("hello\nworld\n"))
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
		s.Write([]byte("short"))
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
		s.Write([]byte("data"))
		s.Close()
	}()

	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 1024)
	if out != "data" {
		t.Errorf("expected 'data', got %q", out)
	}
}

func TestHandleConnectionDotHelp(t *testing.T) {
	s, c := newPipe()
	defer c.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleConnection(s, testDB, "testhost")
	}()

	waitForPrompt(c)

	wc := bufio.NewWriter(c)
	wc.WriteString(".help\n")
	wc.WriteString(".quit\n")
	wc.Flush()

	c.Close()
	wg.Wait()
}

func TestHandleConnectionDotTables(t *testing.T) {
	s, c := newPipe()
	defer c.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleConnection(s, testDB, "testhost")
	}()

	waitForPrompt(c)

	wc := bufio.NewWriter(c)
	wc.WriteString(".tables\n")
	wc.WriteString(".quit\n")
	wc.Flush()

	c.Close()
	wg.Wait()
}

func TestHandleConnectionDotSchema(t *testing.T) {
	s, c := newPipe()
	defer c.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleConnection(s, testDB, "testhost")
	}()

	waitForPrompt(c)

	wc := bufio.NewWriter(c)
	wc.WriteString(".schema\n")
	wc.WriteString(".quit\n")
	wc.Flush()

	c.Close()
	wg.Wait()
}

func TestHandleConnectionDotDatabases(t *testing.T) {
	s, c := newPipe()
	defer c.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleConnection(s, testDB, "testhost")
	}()

	waitForPrompt(c)

	wc := bufio.NewWriter(c)
	wc.WriteString(".databases\n")
	wc.WriteString(".quit\n")
	wc.Flush()

	c.Close()
	wg.Wait()
}

func TestHandleConnectionUnknownCommand(t *testing.T) {
	s, c := newPipe()
	defer c.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleConnection(s, testDB, "testhost")
	}()

	waitForPrompt(c)

	wc := bufio.NewWriter(c)
	wc.WriteString(".nonexistent\n")
	wc.WriteString(".quit\n")
	wc.Flush()

	c.Close()
	wg.Wait()
}

func waitForPrompt(c net.Conn) string {
	r := bufio.NewReader(c)
	var sb strings.Builder
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}
		sb.WriteString(line)
		if strings.HasSuffix(line, "> ") || strings.Contains(line, "> ") {
			break
		}
	}
	return sb.String()
}

func newPipe() (net.Conn, net.Conn) {
	return net.Pipe()
}

func TestRunQuerySelectPragmas(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "PRAGMA table_info(t)", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if !strings.Contains(out.String(), "name") {
		t.Errorf("expected 'name' in pragma output, got %q", out.String())
	}
}

func TestHandleSQLCommandLowercase(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		handleSQLCommand(s, testDB, "select name from t where id=1", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "alice") {
		t.Errorf("expected 'alice', got %q", out)
	}
}

func TestAcceptLoopShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	stop := make(chan os.Signal, 1)
	done := make(chan struct{})
	go func() {
		acceptLoop(listener, testDB, "testhost", stop)
		close(done)
	}()

	stop <- os.Interrupt
	<-done
}

func TestAcceptLoopConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	stop := make(chan os.Signal, 1)
	go acceptLoop(listener, testDB, "testhost", stop)

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	reader := bufio.NewReader(conn)
	// Read welcome
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.Contains(line, "> ") || strings.HasSuffix(line, "> ") {
			break
		}
	}

	writer := bufio.NewWriter(conn)
	writer.WriteString(".quit\n")
	writer.Flush()
	conn.Close()

	stop <- os.Interrupt
}

func TestRunQuerySelectWithWhere(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT name FROM t WHERE value = 2", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "bob") {
		t.Errorf("expected 'bob', got %q", out)
	}
}

func TestRunQuerySelectWithOrderBy(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT name FROM t ORDER BY value DESC", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if strings.TrimSpace(out) == "" {
		t.Error("expected non-empty output")
	}
}

func TestRunQuerySelectWithLimit(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT name FROM t LIMIT 1", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if strings.TrimSpace(out) == "" {
		t.Error("expected non-empty output")
	}
}

func TestRunQuerySelectWithCount(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT COUNT(*) as cnt FROM t", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "3") {
		t.Errorf("expected '3', got %q", out)
	}
}

func TestRunQuerySelectWithGroupBy(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT value, COUNT(*) as cnt FROM t GROUP BY value", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if !strings.Contains(out.String(), "cnt") {
		t.Errorf("expected 'cnt' header, got %q", out.String())
	}
}

func TestHandleSQLCommandUpdate(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		handleSQLCommand(s, testDB, "UPDATE t SET value = 99 WHERE id = 1", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 1024)
	<-done
	if !strings.Contains(out, "OK") || !strings.Contains(out, "1 rows") {
		t.Errorf("expected 'OK. 1 rows affected.', got %q", out)
	}
}

func TestHandleSQLCommandDelete(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		handleSQLCommand(s, testDB, "DELETE FROM t WHERE id = 1", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 1024)
	<-done
	if !strings.Contains(out, "OK") || !strings.Contains(out, "1 rows") {
		t.Errorf("expected 'OK. 1 rows affected.', got %q", out)
	}
}

func TestRunQuerySyntaxError(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELCT * FROM t", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 1024)
	<-done
	if !strings.Contains(out, "Error") {
		t.Errorf("expected 'Error', got %q", out)
	}
}

func TestRunQuerySelectWithJoin(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT t1.name, t2.name FROM t t1, t t2 WHERE t1.id = 1 AND t2.id = 2", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "alice") || !strings.Contains(out, "bob") {
		t.Errorf("expected 'alice' and 'bob', got %q", out)
	}
}

func TestRunQuerySelectWithAggregate(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT COUNT(*) as cnt FROM t WHERE value IS NOT NULL", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "2") {
		t.Errorf("expected '2', got %q", out)
	}
}

func TestRunQuerySelectWithSubquery(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT name FROM t WHERE id IN (SELECT id FROM t WHERE value = 1)", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "alice") {
		t.Errorf("expected 'alice', got %q", out)
	}
}

func TestRunQuerySelectWithLike(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT name FROM t WHERE name LIKE 'a%'", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "alice") {
		t.Errorf("expected 'alice', got %q", out)
	}
}

func TestRunQuerySelectWithBetween(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT name FROM t WHERE value BETWEEN 1 AND 2", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if !strings.Contains(out.String(), "alice") || !strings.Contains(out.String(), "bob") {
		t.Errorf("expected 'alice' and 'bob', got %q", out.String())
	}
}

func TestRunQuerySelectWithIn(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT name FROM t WHERE value IN (1, 3)", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if !strings.Contains(out.String(), "alice") {
		t.Errorf("expected 'alice', got %q", out.String())
	}
}

func TestRunQuerySelectWithIsNull(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT name FROM t WHERE value IS NULL", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if strings.TrimSpace(out) == "" {
		t.Error("expected output")
	}
}

func TestRunQuerySelectWithIsNotNull(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT name FROM t WHERE value IS NOT NULL ORDER BY id", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if !strings.Contains(out.String(), "alice") || !strings.Contains(out.String(), "bob") {
		t.Errorf("expected 'alice' and 'bob', got %q", out.String())
	}
}

func TestRunQuerySelectWithDistinct(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT DISTINCT value FROM t WHERE value IS NOT NULL ORDER BY value", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if !strings.Contains(out.String(), "1") {
		t.Errorf("expected '1' in output, got %q", out.String())
	}
}

func TestRunQuerySelectWithUnion(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT name FROM t WHERE id = 1 UNION ALL SELECT name FROM t WHERE id = 2", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if !strings.Contains(out.String(), "alice") || !strings.Contains(out.String(), "bob") {
		t.Errorf("expected 'alice' and 'bob', got %q", out.String())
	}
}

func TestRunQuerySelectWithAlias(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT name AS user_name FROM t WHERE id = 1", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "alice") {
		t.Errorf("expected 'alice', got %q", out)
	}
}

func TestRunQuerySelectWithMath(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT value + 10 FROM t WHERE id = 1", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "11") {
		t.Errorf("expected '11', got %q", out)
	}
}

func TestRunQuerySelectWithUpper(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT UPPER(name) FROM t WHERE id = 1", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "ALICE") {
		t.Errorf("expected 'ALICE', got %q", out)
	}
}

func TestRunQuerySelectWithCoalesce(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT COALESCE(name, 'unknown') FROM t ORDER BY id", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if !strings.Contains(out.String(), "alice") || !strings.Contains(out.String(), "unknown") {
		t.Errorf("expected 'alice' and 'unknown', got %q", out.String())
	}
}

func TestRunQuerySelectWithCase(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT CASE WHEN value > 1 THEN 'high' ELSE 'low' END as level FROM t ORDER BY id", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if !strings.Contains(out.String(), "low") || !strings.Contains(out.String(), "high") {
		t.Errorf("expected 'low' and 'high', got %q", out.String())
	}
}

func TestRunQuerySelectWithIif(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT IIF(value > 1, 'big', 'small') FROM t ORDER BY id", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if !strings.Contains(out.String(), "small") || !strings.Contains(out.String(), "big") {
		t.Errorf("expected 'small' and 'big', got %q", out.String())
	}
}

func TestHandleConnectionMultipleCommands(t *testing.T) {
	s, c := newPipe()
	defer c.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleConnection(s, testDB, "testhost")
	}()

	waitForPrompt(c)

	wc := bufio.NewWriter(c)
	wc.WriteString("SELECT name FROM t WHERE id=1\n")
	wc.WriteString("INSERT INTO t(name,value) VALUES('new',100)\n")
	wc.WriteString("PRAGMA database_list\n")
	wc.WriteString(".tables\n")
	wc.WriteString(".schema\n")
	wc.WriteString(".databases\n")
	wc.WriteString(".help\n")
	wc.WriteString(".quit\n")
	wc.Flush()

	c.Close()
	wg.Wait()
}

func TestHandleConnectionNonexistentCommand(t *testing.T) {
	s, c := newPipe()
	defer c.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleConnection(s, testDB, "testhost")
	}()

	waitForPrompt(c)

	wc := bufio.NewWriter(c)
	wc.WriteString("INSERT INTO nonexistent(col) VALUES(1)\n")
	wc.WriteString(".quit\n")
	wc.Flush()

	c.Close()
	wg.Wait()
}

func TestRunQuerySelectWithDateFunctions(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT datetime('now')", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if strings.TrimSpace(out) == "" {
		t.Error("expected datetime output")
	}
}

func TestRunQuerySelectWithLength(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT LENGTH(name) FROM t WHERE id = 1", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "5") {
		t.Errorf("expected '5', got %q", out)
	}
}

func TestRunQuerySelectWithAbs(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT ABS(-5)", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "5") {
		t.Errorf("expected '5', got %q", out)
	}
}

func TestRunQuerySelectWithRound(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT ROUND(3.14159, 2)", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "3.14") {
		t.Errorf("expected '3.14', got %q", out)
	}
}

func TestRunQuerySelectWithMaxMin(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT MAX(value), MIN(value) FROM t WHERE value IS NOT NULL", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "3") || !strings.Contains(out, "1") {
		t.Errorf("expected '3' and '1', got %q", out)
	}
}

func TestRunQuerySelectWithSumAvg(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT SUM(value), AVG(value) FROM t WHERE value IS NOT NULL", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "6") || !strings.Contains(out, "2") {
		t.Errorf("expected '6' and '2', got %q", out)
	}
}

func TestRunQuerySelectWithNullIf(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT NULLIF(value, 1) FROM t ORDER BY id", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if !strings.Contains(out.String(), "NULL") {
		t.Errorf("expected NULL, got %q", out.String())
	}
}

func TestRunQuerySelectWithIfNull(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT IFNULL(name, 'no name') FROM t ORDER BY id", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if !strings.Contains(out.String(), "alice") || !strings.Contains(out.String(), "no name") {
		t.Errorf("expected 'alice' and 'no name', got %q", out.String())
	}
}

func TestRunQuerySelectWithTypeof(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT TYPEOF(value) FROM t WHERE id = 1", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "integer") {
		t.Errorf("expected 'integer', got %q", out)
	}
}

func TestRunQuerySelectWithSubstr(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT SUBSTR(name, 1, 3) FROM t WHERE id = 1", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "ali") {
		t.Errorf("expected 'ali', got %q", out)
	}
}

func TestRunQuerySelectWithReplace(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT REPLACE(name, 'alice', 'ALICE') FROM t WHERE id = 1", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "ALICE") {
		t.Errorf("expected 'ALICE', got %q", out)
	}
}

func TestRunQuerySelectWithLower(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT LOWER('HELLO')", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "hello") {
		t.Errorf("expected 'hello', got %q", out)
	}
}

func TestRunQuerySelectWithQuote(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT QUOTE(42)", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "42") {
		t.Errorf("expected '42', got %q", out)
	}
}

func TestRunQuerySelectWithHex(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT HEX('ABC')", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "414243") {
		t.Errorf("expected '414243', got %q", out)
	}
}

func TestRunQuerySelectWithCast(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT CAST(value AS TEXT) FROM t WHERE id = 1", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "1") {
		t.Errorf("expected '1', got %q", out)
	}
}

func TestRunQuerySelectWithConcat(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT name || '_' || value FROM t WHERE id = 1", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "alice_1") {
		t.Errorf("expected 'alice_1', got %q", out)
	}
}

func TestHandleSQLCommandMultipleInsert(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		handleSQLCommand(s, testDB, "INSERT INTO t(name,value) VALUES ('x', 1), ('y', 2), ('z', 3)", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 1024)
	<-done
	if !strings.Contains(out, "OK") || !strings.Contains(out, "3 rows") {
		t.Errorf("expected 'OK. 3 rows affected.', got %q", out)
	}
}

func TestRunQuerySelectWithSelectPragmas(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "PRAGMA compile_options", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if out.Len() == 0 {
		t.Error("expected non-empty pragma output")
	}
}

func TestHandleSQLCommandPragmaLowercase(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		handleSQLCommand(s, testDB, "pragma database_list", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "main") {
		t.Errorf("expected 'main', got %q", out)
	}
}

func TestRunQuerySelectWithCountDistinct(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT COUNT(DISTINCT value) FROM t WHERE value IS NOT NULL", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "3") {
		t.Errorf("expected '3', got %q", out)
	}
}

func TestRunQuerySelectWithHaving(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT value, COUNT(*) as cnt FROM t GROUP BY value HAVING COUNT(*) > 1", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if !strings.Contains(out.String(), "cnt") {
		t.Errorf("expected 'cnt' header, got %q", out.String())
	}
}

func TestRunQuerySelectWithOffset(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT name FROM t ORDER BY id LIMIT 2 OFFSET 1", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if !strings.Contains(out.String(), "bob") {
		t.Errorf("expected 'bob', got %q", out.String())
	}
}

func TestRunQuerySelectWithUnicode(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT UNICODE('A')", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "65") {
		t.Errorf("expected '65', got %q", out)
	}
}

func TestRunQuerySelectWithZeroBlob(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT ZEROBLOB(5)", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if strings.TrimSpace(out) == "" {
		t.Error("expected output for ZEROBLOB")
	}
}

func TestHandleSQLCommandPragmaMultiple(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		handleSQLCommand(s, testDB, "PRAGMA table_info(t)", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if !strings.Contains(out.String(), "name") {
		t.Errorf("expected 'name' in pragma output, got %q", out.String())
	}
}

func TestAcceptLoopMultipleConnections(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	stop := make(chan os.Signal, 1)
	go acceptLoop(listener, testDB, "testhost", stop)

	for i := 0; i < 3; i++ {
		conn, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		reader := bufio.NewReader(conn)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			if strings.Contains(line, "> ") || strings.HasSuffix(line, "> ") {
				break
			}
		}
		writer := bufio.NewWriter(conn)
		writer.WriteString(".quit\n")
		writer.Flush()
		conn.Close()
	}

	stop <- os.Interrupt
}

func TestHandleConnectionConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, c := newPipe()
			defer c.Close()

			var innerWg sync.WaitGroup
			innerWg.Add(1)
			go func() {
				defer innerWg.Done()
				handleConnection(s, testDB, "testhost")
			}()

			waitForPrompt(c)
			wc := bufio.NewWriter(c)
			wc.WriteString("SELECT 1\n")
			wc.WriteString(".quit\n")
			wc.Flush()
			c.Close()
			innerWg.Wait()
		}()
	}
	wg.Wait()
}

func TestHandleConnectionEmptyAndQuit(t *testing.T) {
	s, c := newPipe()
	defer c.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleConnection(s, testDB, "testhost")
	}()

	waitForPrompt(c)

	wc := bufio.NewWriter(c)
	wc.WriteString("\n")
	wc.WriteString("\n")
	wc.WriteString("\n")
	wc.WriteString(".quit\n")
	wc.Flush()

	c.Close()
	wg.Wait()
}

func TestHandleSQLCommandError(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		handleSQLCommand(s, testDB, "UPDATE nonexistent SET col = 1", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 1024)
	<-done
	if !strings.Contains(out, "Error") {
		t.Errorf("expected 'Error', got %q", out)
	}
}

func TestRunQuerySelectWithHavingMultiple(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT value, COUNT(*) as cnt FROM t GROUP BY value HAVING COUNT(*) > 1 AND MAX(value) > 0", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if !strings.Contains(out.String(), "cnt") {
		t.Errorf("expected 'cnt' header, got %q", out.String())
	}
}

func TestRunQuerySelectWithMultipleGroupBy(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT value, COUNT(*) as cnt, MAX(name) as max_name FROM t GROUP BY value ORDER BY value", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if !strings.Contains(out.String(), "cnt") {
		t.Errorf("expected 'cnt' header, got %q", out.String())
	}
}

func TestRunQuerySelectWithIifMultiple(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT IIF(value > 1, 'big', 'small'), IIF(name = 'alice', 'first', 'other') FROM t ORDER BY id", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	sout := out.String()
	if !strings.Contains(sout, "small") || !strings.Contains(sout, "big") || !strings.Contains(sout, "first") {
		t.Errorf("expected 'small', 'big', and 'first', got %q", sout)
	}
}

func TestRunQuerySelectWithNestedCase(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT CASE WHEN value > 2 THEN 'high' WHEN value > 1 THEN 'medium' ELSE 'low' END as tier FROM t ORDER BY id", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	sout := out.String()
	if !strings.Contains(sout, "low") || !strings.Contains(sout, "medium") || !strings.Contains(sout, "high") {
		t.Errorf("expected 'low', 'medium', and 'high', got %q", sout)
	}
}

func TestRunQuerySelectWithExists(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT name FROM t t1 WHERE EXISTS (SELECT 1 FROM t t2 WHERE t2.id = t1.id AND t2.value = 1)", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "alice") {
		t.Errorf("expected 'alice', got %q", out)
	}
}

func TestRunQuerySelectWithNotExists(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT name FROM t t1 WHERE NOT EXISTS (SELECT 1 FROM t t2 WHERE t2.id = t1.id AND t2.value = 999)", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	sout := out.String()
	if !strings.Contains(sout, "alice") || !strings.Contains(sout, "bob") {
		t.Errorf("expected 'alice' and 'bob', got %q", sout)
	}
}

func TestRunQuerySelectWithCoalesceNullif(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT COALESCE(NULLIF(name, 'alice'), 'not alice') FROM t ORDER BY id", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	sout := out.String()
	if !strings.Contains(sout, "not alice") || !strings.Contains(sout, "bob") {
		t.Errorf("expected 'not alice' and 'bob', got %q", sout)
	}
}

func TestRunQuerySelectWithIfnullComplex(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT IFNULL(NULLIF(value, 1), -1) FROM t ORDER BY id", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	sout := out.String()
	if !strings.Contains(sout, "-1") {
		t.Errorf("expected '-1', got %q", sout)
	}
}

func TestRunQuerySelectWithTypeofMultiple(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT TYPEOF(name), TYPEOF(value) FROM t WHERE id = 1", w)
	}()
	r := bufio.NewReader(c)
	out := readUntil(r, '\n', 4096)
	<-done
	if !strings.Contains(out, "text") || !strings.Contains(out, "integer") {
		t.Errorf("expected 'text' and 'integer', got %q", out)
	}
}

func TestRunQuerySelectWithNullifComplex(t *testing.T) {
	s, c := newPipe()
	defer s.Close()
	defer c.Close()
	w := tabwriter.NewWriter(s, 0, 0, 2, ' ', 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runQuery(s, testDB, "SELECT NULLIF(value, value) FROM t ORDER BY id", w)
	}()
	r := bufio.NewReader(c)
	var out strings.Builder
	for {
		line, err := r.ReadString('\n')
		out.WriteString(line)
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	<-done
	if !strings.Contains(out.String(), "NULL") {
		t.Errorf("expected NULL, got %q", out.String())
	}
}

// Suppress unused import
var _ = io.EOF
