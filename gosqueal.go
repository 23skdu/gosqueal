package main
import ( "net"
         "os"
	 "database/sql"
	_ "modernc.org/sqlite"
        "github.com/rs/zerolog/log"
)
const (
        SERVER_HOST = "0.0.0.0"
        SERVER_PORT = "1118"
        SERVER_TYPE = "tcp"
)
func main() {
  hostname, err := os.Hostname()
  if err != nil { panic(err)
                  os.Exit(1) }
  log.Info().Msg(hostname)
  db, err := sql.Open("sqlite",":memory:")
  if err != nil { panic(err)
                  os.Exit(1) }
  db.Exec(`
		create table metrics(metricname text primary key, time timestamp, value real);
		create table translog(time timestamp, query text);
	`)
  log.Info().Msg("Start server...")
  server, err := net.Listen(SERVER_TYPE, SERVER_HOST+":"+SERVER_PORT)
        if err != nil {
                log.Info().Msg("Error listening:"+err.Error())
                os.Exit(1)
        }
        defer server.Close()
        log.Info().Msg("Listening on " + SERVER_HOST + ":" + SERVER_PORT)
        log.Info().Msg("Waiting for client...")
        for {
                connection, err := server.Accept()
                if err != nil {
                        log.Info().Msg("Error accepting: "+err.Error())
                        os.Exit(1)
                }
                log.Info().Msg("client connected")
                go processClient(connection)
        }
}
func processClient(connection net.Conn) {
        buffer := make([]byte, 1024)
        mLen, err := connection.Read(buffer)
        if err != nil {
                log.Info().Msg("Error reading:"+err.Error())
        }
        log.Info().Msg("Received: "+string(buffer[:mLen]))
        _, err = connection.Write([]byte("Thanks! Got your message:" + string(buffer[:mLen])))
        connection.Close()
}
