#Golang SQLite3 In Memory "HotPocket"
```
gosqueal $ docker build . -t gosqueal:0.1
[+] Building 0.1s (11/11) FINISHED                                                                                           docker:desktop-linux
 => [internal] load .dockerignore                                                                                                            0.0s
 => => transferring context: 2B                                                                                                              0.0s
 => [internal] load build definition from Dockerfile                                                                                         0.0s
 => => transferring dockerfile: 247B                                                                                                         0.0s
 => [internal] load metadata for docker.io/library/debian:trixie-slim                                                                        0.0s
 => [internal] load metadata for docker.io/library/golang:latest                                                                             0.0s
 => [internal] load build context                                                                                                            0.0s
 => => transferring context: 85B                                                                                                             0.0s
 => [stage-1 1/2] FROM docker.io/library/debian:trixie-slim                                                                                  0.0s
 => [builder 1/4] FROM docker.io/library/golang:latest                                                                                       0.0s
 => CACHED [builder 2/4] COPY go.mod go.sum gosqueal.go /                                                                                    0.0s
 => CACHED [builder 3/4] RUN set -x    && GOOS=linux go build gosqueal.go                                                                    0.0s
 => [stage-1 2/2] COPY --from=builder /gosqueal /gosqueal                                                                                    0.0s
 => exporting to image                                                                                                                       0.0s
 => => exporting layers                                                                                                                      0.0s
 => => writing image sha256:a129f579c07be446af36ae97645997149825aa5f5767e84ce1c9808465f6fcf9                                                 0.0s
 => => naming to docker.io/library/gosqueal:0.1                                                         
```
---
```
gosqueal $ docker run -p 1118:1118 gosqueal:0.1
Start server...
Listening on 0.0.0.0:1118
Waiting for client...
```
