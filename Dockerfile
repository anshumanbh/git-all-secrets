FROM golang:latest
MAINTAINER Anshuman Bhartiya <anshuman.bhartiya@gmail.com>

ADD . /data
WORKDIR /data/thog

RUN apt-get update && apt-get install -y python-pip jq
RUN pip install -r requirements.txt
RUN chmod +x truffleHog/truffleHog.py

WORKDIR /data
RUN chmod +x rungitsecrets.sh
RUN chmod +x runreposupervisor.sh
RUN git clone https://github.com/anshumanbh/git-secrets.git && cd git-secrets && make install

RUN git clone https://github.com/anshumanbh/repo-supervisor.git

WORKDIR /data/repo-supervisor

RUN curl -o- https://raw.githubusercontent.com/creationix/nvm/v0.33.2/install.sh | bash
RUN /bin/bash -c "source ~/.bashrc && nvm install 7"
RUN /bin/bash -c "source ~/.bashrc && cd /data/repo-supervisor && npm install --no-optional && npm run build"

WORKDIR /data

RUN go get github.com/google/go-github/github && go get github.com/satori/go.uuid && go get golang.org/x/oauth2
RUN go build -o gitallsecrets .

ENTRYPOINT ["./gitallsecrets"]
