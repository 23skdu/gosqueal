#Golang SQLite3 In Memory "HotPocket"
## teeny tiny small Docker image
```
$ docker build . -t gosqueal:0.1
```
you can pass custom ip and ports on the cmdline
```
$ docker run -p 8080:8080 gosqueal:0.1 /gosqueal -port=8080 -host 127.0.0.1
{"level":"info","time":"2024-01-10T06:11:55Z","message":"717684fa40fa:: listening on 127.0.0.1:8080"}
```
