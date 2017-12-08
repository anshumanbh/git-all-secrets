FROM golang:latest
MAINTAINER Anshuman Bhartiya <anshuman.bhartiya@gmail.com>

COPY main.go /data/main.go
COPY rungitsecrets.sh /data/rungitsecrets.sh
COPY runreposupervisor.sh /data/runreposupervisor.sh

RUN apt-get update && apt-get install -y python-pip jq

WORKDIR /data
RUN git clone https://github.com/dxa4481/truffleHog.git
COPY regexChecks.py /data/truffleHog/truffleHog/regexChecks.py
COPY requirements.txt /data/truffleHog/requirements.txt
RUN pip install -r /data/truffleHog/requirements.txt

# create a generic SSH config for Github
WORKDIR /root/.ssh
RUN echo "Host *github.com \
\n  IdentitiesOnly yes \
\n  StrictHostKeyChecking no \
\n  UserKnownHostsFile=/dev/null \
\n  IdentityFile /root/.ssh/id_rsa" > config

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
