#!/bin/bash

JSON_OUTPUT=1 /root/.nvm/versions/node/v7.10.1/bin/node ./repo-supervisor/dist/cli.js $1 > $2

exit 0
