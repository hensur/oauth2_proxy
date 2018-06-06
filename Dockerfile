FROM golang:1.10.2
RUN mkdir -p /go/src/github.com/bitly
RUN cd /go/src/github.com/bitly && git clone https://github.com/hensur/oauth2_proxy.git \
    && curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
WORKDIR /go/src/github.com/bitly/oauth2_proxy
RUN dep ensure
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o oauth2_proxy .

FROM alpine:latest
LABEL maintainer="Henning Surmeier <me@hensur.de>"
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=0 /go/src/github.com/bitly/oauth2_proxy/oauth2_proxy .

EXPOSE 4180
ENTRYPOINT [ "/root/oauth2_proxy" ]
CMD [ "--upstream=http://0.0.0.0:8080/", "--http-address=0.0.0.0:4180" ]