package main

import (
"database/sql"
"flag"
"net"
"os"
"os/signal"
"syscall"

"github.com/rs/zerolog"
"github.com/rs/zerolog/log"
_ "modernc.org/sqlite"
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

db, err := sql.Open("sqlite", ":memory:")
if err != nil {
log.Fatal().Err(err).Msg("failed to open db")
}
defer db.Close()

_, err = db.Exec(`
create table translog(time timestamp primary key, client text, query text);
`)
if err != nil {
log.Fatal().Err(err).Msg("failed to create table")
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

buffer := make([]byte, 1024)
mLen, err := conn.Read(buffer)
if err != nil {
log.Error().Err(err).Msg("error reading")
return
}

query := string(buffer[:mLen])
log.Info().Str("client", ip).Str("query", query).Msg("received query")

_, err = db.Exec("INSERT into translog VALUES(TIME('now'),?,?);", ip, query)
if err != nil {
log.Error().Err(err).Msg("error logging transaction")
}

// Execute the query
_, err = db.Exec(query)
if err != nil {
log.Error().Err(err).Str("query", query).Msg("error executing query")
conn.Write([]byte("Error: " + err.Error() + "\n"))
} else {
conn.Write([]byte("OK\n"))
}
}
