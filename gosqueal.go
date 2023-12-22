package main
import ( "net"
         "os"
         "log"
         "fmt"
         "bytes"
	 "database/sql"
	_ "modernc.org/sqlite"
)
const (
        SERVER_HOST = "0.0.0.0"
        SERVER_PORT = "1118"
        SERVER_TYPE = "tcp"
)
func main() {
  hostname, err := os.Hostname()
  var (
  buf    bytes.Buffer
  logger = log.New(&buf, hostname, log.Lshortfile)
  )
  db, err := sql.Open("sqlite",":memory:")
  if err != nil { panic(err)
                  os.Exit(1) }
  db.Exec(`
		create table metrics(metricname text primary key, time timestamp, value real);
		create table translog(time timestamp, query text);
	`)
  logger.Print("Start server...")
  server, err := net.Listen(SERVER_TYPE, SERVER_HOST+":"+SERVER_PORT)
        if err != nil {
                logger.Print("Error listening:", err.Error())
                os.Exit(1)
        }
        defer server.Close()
        logger.Print("Listening on " + SERVER_HOST + ":" + SERVER_PORT)
        logger.Print("Waiting for client...")
        for {
                connection, err := server.Accept()
                if err != nil {
                        logger.Print("Error accepting: ", err.Error())
                        os.Exit(1)
                }
                logger.Print("client connected")
                go processClient(connection)
        }
}
func processClient(connection net.Conn) {
        buffer := make([]byte, 1024)
        mLen, err := connection.Read(buffer)
        if err != nil {
                fmt.Println("Error reading:", err.Error())
        }
        fmt.Println("Received: ", string(buffer[:mLen]))
        _, err = connection.Write([]byte("Thanks! Got your message:" + string(buffer[:mLen])))
        connection.Close()
}
