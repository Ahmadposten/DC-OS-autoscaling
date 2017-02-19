FROM alpine:3.3

MAINTAINER Ahmad Hassan "ahassan@instabug.com"

ENV dir /srv/marathon-autoscaling
ENV GOROOT /usr/lib/go
ENV GOPATH $dir
ENV GOBIN $GOPATH/bin
ENV PATH $PATH:$GOROOT/bin:$GOPATH/bin

RUN mkdir -p $dir

WORKDIR $dir
Add . $dir

RUN apk update && \
    apk add --no-cache git bzr go && go build src/dcos-autoscaling.go && \
    apk del git bzr go python git && \
    rm -rf $dir/src $dir/pkg

CMD ["./dcos-autoscaling"]
