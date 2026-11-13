package main

import (
	"bufio"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	fs := flag.NewFlagSet("gosqueal", flag.ContinueOnError)
	srvHost := fs.String("host", "0.0.0.0", "server ip")
	srvPort := fs.String("port", "1118", "server port")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if len(fs.Args()) > 0 {
		return 1
	}

	if envHost := os.Getenv("HOST"); envHost != "" {
		*srvHost = envHost
	}
	if envPort := os.Getenv("PORT"); envPort != "" {
		*srvPort = envPort
	}

	db, err := initDB()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize database")
		return 1
	}
	defer db.Close()

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get hostname")
		return 1
	}

	addr := *srvHost + ":" + *srvPort
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal().Err(err).Str("hostname", hostname).Msg("error listening")
		return 1
	}

	log.Info().Str("hostname", hostname).Str("addr", addr).Msg("listening")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go acceptLoop(listener, db, hostname, stop)

	<-stop
	log.Info().Msg("shutting down...")
	_ = listener.Close()
	return 0
}

func initDB() (*sql.DB, error) {
	extensions := []string{}
	for _, p := range []string{"/usr/lib/vector0", "/usr/lib/vss0"} {
		if _, err := os.Stat(p); err == nil {
			extensions = append(extensions, p)
		}
	}

	sql.Register("sqlite3_custom", &sqlite3.SQLiteDriver{
		Extensions: extensions,
	})

	db, err := sql.Open("sqlite3_custom", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}
	db.SetMaxOpenConns(1)

	initSQL := `
	CREATE TABLE translog(id INTEGER PRIMARY KEY AUTOINCREMENT, time timestamp, client text, query text);
	`
	if len(extensions) == 2 {
		initSQL += `
	CREATE VIRTUAL TABLE vectors USING vss0(
		headline_embedding(384),
		description_embedding(384)
	);
	`
	}
	if _, err = db.Exec(initSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	return db, nil
}

func acceptLoop(listener net.Listener, db *sql.DB, hostname string, stop <-chan os.Signal) {
	sem := make(chan struct{}, 100)
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-stop:
				return
			default:
				log.Error().Err(err).Msg("error accepting connection")
				continue
			}
		}
		select {
		case sem <- struct{}{}:
		default:
			_ = conn.Close()
			continue
		}
		go func(c net.Conn) {
			defer func() { <-sem }()
			handleConnection(c, db, hostname)
		}(conn)
	}
}

func handleConnection(conn net.Conn, db *sql.DB, hostname string) {
	defer conn.Close()
	ip := conn.RemoteAddr().String()
	log.Info().Str("hostname", hostname).Str("client", ip).Msg("client connected")

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	w := tabwriter.NewWriter(conn, 0, 0, 2, ' ', 0)

	fmt.Fprintf(conn, "Connected to gosqueal at %s\nType .help for usage.\n> ", hostname)

	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			fmt.Fprint(conn, "> ")
			continue
		}

		log.Info().Str("client", ip).Str("query", input).Msg("received query")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := db.ExecContext(ctx, "INSERT INTO translog(time, client, query) VALUES(TIME('now'),?,?);", ip, input)
		cancel()
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
	if len(parts) == 0 {
		return false
	}
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
	upper := strings.ToUpper(strings.TrimSpace(query))
	if strings.HasPrefix(upper, "SELECT") || strings.HasPrefix(upper, "PRAGMA") ||
		strings.HasPrefix(upper, "WITH") || strings.HasPrefix(upper, "EXPLAIN") {
		runQuery(conn, db, query, w)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		res, err := db.ExecContext(ctx, query)
		if err != nil {
			fmt.Fprintf(conn, "Error: %s\n", err)
			return
		}
		rows, _ := res.RowsAffected()
		fmt.Fprintf(conn, "OK. %d rows affected.\n", rows)
	}
}

func runQuery(conn net.Conn, db *sql.DB, query string, w *tabwriter.Writer) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := db.QueryContext(ctx, query)
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

	values := make([]interface{}, len(cols))
	scanTargets := make([]interface{}, len(cols))
	for i := range values {
		scanTargets[i] = &values[i]
	}

	rowCount := 0
	for rows.Next() {
		if rowCount >= 1000 {
			fmt.Fprintln(w, "... (truncated after 1000 rows)")
			break
		}
		rowCount++
		if err := rows.Scan(scanTargets...); err != nil {
			fmt.Fprintf(conn, "Error scanning: %s\n", err)
			return
		}
		for i, v := range values {
			if v == nil {
				fmt.Fprint(w, "NULL")
			} else {
				fmt.Fprintf(w, "%v", v)
			}
			if i < len(cols)-1 {
				fmt.Fprint(w, "\t")
			}
		}
		fmt.Fprintln(w)
	}
	_ = w.Flush()
}
