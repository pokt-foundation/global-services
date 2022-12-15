FROM golang:1.19.3-alpine3.16 AS builder
WORKDIR /app

COPY . .

RUN go mod vendor
RUN cd fishermen/cmd/run-application-checks/cli && go build main.go
RUN cd global-dispatcher/cmd/dispatch/cli/ && go build main.go

FROM alpine:3.17

WORKDIR /app
COPY --chown=65534:65534 --from=builder /app/fishermen/cmd/run-application-checks/cli/main.go ./fishermen/main
COPY --chown=65534:65534 --from=builder /app/global-dispatcher/cmd/dispatch/cli/main.go ./fishermen/main

USER 65534

#ENTRYPOINT [ "/app/fishermen/main" ]