
build:
	go build -o ./flit/build/flit ./flit

clean: 
	rm -r ./flit/build
	rm -r ./flit/dist

install:
	go install ./flit

uninstall:
	rm -r $(HOME)/go/bin/flit

release:
	cd flit; goreleaser release