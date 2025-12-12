package main

import (
"bufio"
"database/sql"
"flag"
"fmt"
"net"
"os"
"os/signal"
"strings"
"syscall"
"text/tabwriter"

"github.com/mattn/go-sqlite3"
"github.com/rs/zerolog"
"github.com/rs/zerolog/log"
)

func main() {
// Setup logging
zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

srvHost := flag.String("host", "0.0.0.0", "server ip")
srvPort := flag.String("port", "1118", "server port")
flag.Parse()

// Allow env var override
if envHost := os.Getenv("HOST"); envHost != "" {
*srvHost = envHost
}
if envPort := os.Getenv("PORT"); envPort != "" {
*srvPort = envPort
}

hostname, err := os.Hostname()
if err != nil {
log.Fatal().Err(err).Msg("failed to get hostname")
}

// Register custom driver to load extensions
sql.Register("sqlite3_custom", &sqlite3.SQLiteDriver{
Extensions: []string{
"/usr/lib/vector0",
"/usr/lib/vss0",
},
})

db, err := sql.Open("sqlite3_custom", ":memory:")
if err != nil {
log.Fatal().Err(err).Msg("failed to open db")
}
defer db.Close()

// Initialize tables
initSQL := `
CREATE TABLE translog(time timestamp primary key, client text, query text);
CREATE VIRTUAL TABLE vectors USING vss0(
headline_embedding(384),
description_embedding(384)
);
`
_, err = db.Exec(initSQL)
if err != nil {
log.Fatal().Err(err).Msg("failed to initialize tables (vss loaded?)")
}

addr := *srvHost + ":" + *srvPort
listener, err := net.Listen("tcp", addr)
if err != nil {
log.Fatal().Err(err).Str("hostname", hostname).Msg("error listening")
}

log.Info().Str("hostname", hostname).Str("addr", addr).Msg("listening")

// Graceful shutdown channel
stop := make(chan os.Signal, 1)
signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

go func() {
for {
conn, err := listener.Accept()
if err != nil {
select {
case <-stop:
return // shutting down
default:
log.Error().Err(err).Msg("error accepting connection")
continue
}
}
go handleConnection(conn, db, hostname)
}
}()

<-stop
log.Info().Msg("shutting down...")
listener.Close()
}

func handleConnection(conn net.Conn, db *sql.DB, hostname string) {
defer conn.Close()
ip := conn.RemoteAddr().String()
log.Info().Str("hostname", hostname).Str("client", ip).Msg("client connected")

scanner := bufio.NewScanner(conn)
w := tabwriter.NewWriter(conn, 0, 0, 2, ' ', 0)

fmt.Fprintf(conn, "Connected to gosqueal at %s\nType .help for usage.\n> ", hostname)

for scanner.Scan() {
input := strings.TrimSpace(scanner.Text())
if input == "" {
fmt.Fprint(conn, "> ")
continue
}

// Log the query
log.Info().Str("client", ip).Str("query", input).Msg("received query")
_, err := db.Exec("INSERT into translog VALUES(TIME('now'),?,?);", ip, input)
if err != nil {
log.Error().Err(err).Msg("error logging transaction")
}

if strings.HasPrefix(input, ".") {
if handleDotCommand(conn, db, input, w) {
return
}
} else {
handleSQLCommand(conn, db, input, w)
}

fmt.Fprint(conn, "> ")
}
}

func handleDotCommand(conn net.Conn, db *sql.DB, input string, w *tabwriter.Writer) bool {
parts := strings.Fields(input)
cmd := parts[0]

switch cmd {
case ".quit", ".exit":
fmt.Fprintln(conn, "Bye!")
return true
case ".tables":
runQuery(conn, db, "SELECT name FROM sqlite_master WHERE type='table'", w)
case ".schema":
runQuery(conn, db, "SELECT sql FROM sqlite_master WHERE sql IS NOT NULL", w)
case ".databases":
runQuery(conn, db, "PRAGMA database_list", w)
case ".help":
fmt.Fprintln(conn, ".tables List tables")
fmt.Fprintln(conn, ".schema Show schema")
fmt.Fprintln(conn, ".databases List databases")
fmt.Fprintln(conn, ".quit Exit")
default:
fmt.Fprintf(conn, "Unknown command: %s\n", cmd)
}
return false
}

func handleSQLCommand(conn net.Conn, db *sql.DB, query string, w *tabwriter.Writer) {
upper := strings.ToUpper(query)
if strings.HasPrefix(upper, "SELECT") || strings.HasPrefix(upper, "PRAGMA") {
runQuery(conn, db, query, w)
} else {
res, err := db.Exec(query)
if err != nil {
fmt.Fprintf(conn, "Error: %s\n", err)
return
}
rows, _ := res.RowsAffected()
fmt.Fprintf(conn, "OK. %d rows affected.\n", rows)
}
}

func runQuery(conn net.Conn, db *sql.DB, query string, w *tabwriter.Writer) {
rows, err := db.Query(query)
if err != nil {
fmt.Fprintf(conn, "Error: %s\n", err)
return
}
defer rows.Close()

cols, _ := rows.Columns()
for i, col := range cols {
fmt.Fprintf(w, "%s", col)
if i < len(cols)-1 {
fmt.Fprint(w, "\t")
}
}
fmt.Fprintln(w)

readCols := make([]interface{}, len(cols))
writeCols := make([]sql.NullString, len(cols))
for i := range writeCols {
readCols[i] = &writeCols[i]
}

for rows.Next() {
err := rows.Scan(readCols...)
if err != nil {
fmt.Fprintf(conn, "Error scanning: %s\n", err)
return
}
for i, col := range writeCols {
if col.Valid {
fmt.Fprintf(w, "%s", col.String)
} else {
fmt.Fprint(w, "NULL")
}
if i < len(cols)-1 {
fmt.Fprint(w, "\t")
}
}
fmt.Fprintln(w)
}
w.Flush()
}
