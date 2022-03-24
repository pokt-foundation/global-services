.PHONY: build clean deploy gomodgen

build: gomodgen 
ifndef NAME
$(error NAME is not set)
endif
	export GO111MODULE=on
	env GOARCH=amd64 GOOS=linux go build -ldflags="-s -w" -o bin/$(NAME) cmd/functions/$(NAME)/main.go

clean:
	rm -rf ./bin ./vendor go.sum

deploy: clean build
	sls deploy --verbose

gomodgen:
	chmod u+x gomod.sh
	./gomod.sh
