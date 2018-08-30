# git-all-secrets build container
FROM golang:1.10.3-alpine3.7 AS build-env

RUN apk add --no-cache --upgrade git openssh-client ca-certificates
RUN go get -u github.com/golang/dep/cmd/dep

WORKDIR /go/src/github.com/anshumanbh/git-all-secrets
COPY Gopkg.toml Gopkg.lock ./
RUN dep ensure -vendor-only -v
COPY main.go ./
RUN go build -v -o /go/bin/git-all-secrets

# Final container
FROM node:9.11.2-alpine

COPY --from=build-env /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build-env /go/bin/git-all-secrets /usr/bin/git-all-secrets
RUN apk add --no-cache --upgrade git python py-pip jq openssh-client
ENV PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

# Create a generic SSH config for Github
WORKDIR /root/.ssh
RUN echo "Host *github.com \
\n  IdentitiesOnly yes \
\n  StrictHostKeyChecking no \
\n  UserKnownHostsFile=/dev/null \
\n  IdentityFile /root/.ssh/id_rsa \
\n  \
\n Host github.*.com \
\n  IdentitiesOnly yes \
\n  StrictHostKeyChecking no \
\n  UserKnownHostsFile=/dev/null \
\n  IdentityFile /root/.ssh/id_rsa" > config
RUN ssh-keyscan -H github.com >> /root/.ssh/known_hosts
RUN git clone https://github.com/anshumanbh/repo-supervisor.git /root/repo-supervisor

# Install truffleHog
WORKDIR /root/truffleHog/
RUN pip install truffleHog
COPY rules.json /root/truffleHog/

# Install repo-supervisor
WORKDIR /root/repo-supervisor
COPY runreposupervisor.sh ./
RUN chmod +x runreposupervisor.sh
RUN npm install --no-optional && \
    npm run build && \
    npm run cli ./src/

ENV PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
WORKDIR /root/
ENTRYPOINT [ "git-all-secrets" ]
