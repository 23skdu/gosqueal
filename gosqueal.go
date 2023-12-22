package main
import ( "net"
         "fmt"
         "os"
	 "database/sql"
	_ "modernc.org/sqlite"
)
const (
        SERVER_HOST = "0.0.0.0"
        SERVER_PORT = "1118"
        SERVER_TYPE = "tcp"
)
func main() {
  db, err := sql.Open("sqlite",":memory:")
  if err != nil { panic(err)
                  os.Exit(1) }
  db.Exec(`
		create table metrics(metricname text primary key, time timestamp, value real);
		create table translog(time timestamp, query text);
	`)
  fmt.Println("Start server...")
  server, err := net.Listen(SERVER_TYPE, SERVER_HOST+":"+SERVER_PORT)
        if err != nil {
                fmt.Println("Error listening:", err.Error())
                os.Exit(1)
        }
        defer server.Close()
        fmt.Println("Listening on " + SERVER_HOST + ":" + SERVER_PORT)
        fmt.Println("Waiting for client...")
        for {
                connection, err := server.Accept()
                if err != nil {
                        fmt.Println("Error accepting: ", err.Error())
                        os.Exit(1)
                }
                fmt.Println("client connected")
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
