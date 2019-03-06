FROM churp/base:latest

COPY . /go/src/mpss

WORKDIR /go/src/mpss/cmd/schultz

RUN go get github.com/BurntSushi/toml \
	github.com/golang/protobuf/proto \
	github.com/montanaflynn/stats github.com/ncw/gmp \
	github.com/sirupsen/logrus \
	github.com/rifflock/lfshook \
	google.golang.org/grpc \
	github.com/docopt/docopt-go

RUN make && mv primary.exe /primary && mv node.exe /node
RUN rm -rf /go/src/mpss

EXPOSE 8000
