FROM google/golang
WORKDIR /gopath/src/github.com/nytlabs/streamtools
ADD . /gopath/src/github.com/nytlabs/streamtools
RUN make
RUN ["mkdir", "-p", "/gopath/bin"]
RUN ["ln", "-s", "/gopath/src/github.com/nytlabs/streamtools/build/st", "/gopath/bin/st"]

EXPOSE 7070

WORKDIR /gopath/bin
CMD ./st
