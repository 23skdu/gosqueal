package main
import ( "net"
         "os"
	 "flag"
	 "database/sql"
         _ "modernc.org/sqlite"
         "github.com/rs/zerolog/log"
)
func main() {
  srvHost := flag.String("host", "0.0.0.0", "server ip")
  srvPort := flag.String("port", "1118", "server port")
  flag.Parse()
  hostname, err := os.Hostname()
  if err != nil { panic(err)
                  os.Exit(1) }
  db, err := sql.Open("sqlite",":memory:")
  if err != nil { panic(err)
                  os.Exit(1) }
  db.Exec(`
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
                buffer := make([]byte, 1024)
                mLen, err := connection.Read(buffer)
                if err != nil {
                log.Info().Msg("Error reading:"+err.Error())
                }
                log.Info().Msg("From "+ip+" Received: "+string(buffer[:mLen]))
                _, err = db.Exec("INSERT into translog VALUES(TIME('now'),?,?);", ip, string(buffer[:mLen]))
                if err != nil {
			log.Info().Msg("error: "+err.Error())
                }
                xxc, err = db.Exec(string(buffer[:mLen]))
                if err != nil {
                log.Info().Msg("error: "+err.Error())
                }
		_, err = connection.Write([]byte(xxc.Result()))
                connection.Close()
	}
}
