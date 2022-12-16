FROM golang:1.19.3-alpine3.16 AS builder
WORKDIR /app

COPY . .

RUN go mod vendor
ENV  GO111MODULE=on
RUN cd fishermen/cmd/run-application-checks/cli && GOARCH=amd64 GOOS=linux go build main.go
RUN cd global-dispatcher/cmd/dispatch/cli/ && GOARCH=amd64 GOOS=linux go build main.go

FROM alpine:3.17
WORKDIR /app

COPY --from=builder /app/fishermen/cmd/run-application-checks/cli/main.go ./fishermen/main
COPY --from=builder /app/global-dispatcher/cmd/dispatch/cli/main.go ./dispatch/main

RUN chmod +x ./fishermen/main 
RUN chmod +x ./dispatch/main

#ENTRYPOINT [ "/app/fishermen/main" ]
