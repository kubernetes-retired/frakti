all: frakti

frakti: frakti.go $(wildcard **/**.go)
	go build frakti.go

install:
	cp -f frakti /usr/local/bin

clean:
	rm -f frakti
