.PHONY: build clean deploy gomodgen

build: gomodgen 
ifndef SERVICE
$(error SERVICE is not set)
endif
ifndef NAME
$(error NAME is not set)
endif

	export GO111MODULE=on
	env GOARCH=amd64 GOOS=linux go build -ldflags="-s -w" -o bin/$(NAME) src/cmd/$(SERVICE)/$(NAME)/lambda/main.go

clean:
	rm -rf ./bin ./vendor go.sum

deploy: clean build
	sls deploy --verbose

gomodgen:
	chmod u+x gomod.sh
	./gomod.sh
