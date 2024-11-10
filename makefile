
build:
	go build -o ./flit/build/flit ./flit

clean: 
	rm -r ./flit/build

install:
	go install ./flit

uninstall:
	rm -r $(HOME)/go/bin/flit