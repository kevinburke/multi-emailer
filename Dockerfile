FROM golang
ADD . /go/src/github.com/skylartaylor/multi-emailer
#install mailerapp
RUN go get github.com/skylartaylor/multi-emailer && go install github.com/skylartaylor/multi-emailer

#command setup
ENTRYPOINT ["multi-emailer", "--config", "/go/src/github.com/skylartaylor/multi-emailer/config.yml"]
