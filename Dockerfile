FROM golang
EXPOSE 8480
ENTRYPOINT ["make", "watch"]

RUN go get github.com/skylartaylor/multi-emailer
WORKDIR /go/src/github.com/skylartaylor/multi-emailer
ADD . .
#install mailerapp
RUN make generate_cert && go get
