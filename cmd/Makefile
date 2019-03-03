all: node primary protocol

clean:
	rm -rf *.exe

node:
	go build -o node.exe node.go init.go

primary:
	go build -o primary.exe primary.go init.go

protocol:
	go build -o protocol.exe protocol.go init.go

