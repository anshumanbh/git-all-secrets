FROM golang:latest
MAINTAINER Anshuman Bhartiya <anshuman.bhartiya@gmail.com>

ADD . /data
WORKDIR /data/thog

RUN apt-get update && apt-get install -y python-pip
RUN pip install -r requirements.txt
RUN chmod +x truffleHog/truffleHog.py

WORKDIR /data
RUN chmod +x rungitsecrets.sh
RUN git clone https://github.com/anshumanbh/git-secrets.git && cd git-secrets && make install

RUN go get github.com/google/go-github/github && go get github.com/satori/go.uuid && go get golang.org/x/oauth2
RUN go build -o gitallsecrets .

ENTRYPOINT ["./gitallsecrets"]