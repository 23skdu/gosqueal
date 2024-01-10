package main
import ( "net"
         "os"
	 "flag"
	 "database/sql"
	_ "modernc.org/sqlite"
        "github.com/rs/zerolog/log"
)
func main() {
  srvHost := flag.String("host", "0.0.0.0", "server host ip")
  srvPort := flag.String("port", "1118", "server host ip")
  flag.Parse()
  hostname, err := os.Hostname()
  if err != nil { panic(err)
                  os.Exit(1) }
  db, err := sql.Open("sqlite",":memory:")
  if err != nil { panic(err)
                  os.Exit(1) }
  db.Exec(`
		create table metrics(metricname text primary key, time timestamp, value real);
		create table translog(time timestamp primary key, client text, query text);
	`)
  server, err := net.Listen("tcp", *srvHost +":"+ *srvPort)
        if err != nil {
		log.Error().Msg(hostname+":: error listening:"+err.Error())
                os.Exit(1)
        }
        defer server.Close()
	log.Info().Msg(hostname+":: listening on " + *srvHost + ":" + *srvPort)
        for {
                connection, err := server.Accept()
                if err != nil {
                        log.Info().Msg("Error accepting: "+err.Error())
                        os.Exit(1)
                }
		ip := connection.RemoteAddr().String()
		log.Info().Msg(hostname+":: client connected from "+ip)
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

func getEnv(key, fallback string) string {
    value, exists := os.LookupEnv(key)
    if !exists {
        value = fallback
    }
    return value
}
