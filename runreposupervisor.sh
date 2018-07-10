#!/bin/sh

JSON_OUTPUT=1 node ./repo-supervisor/dist/cli.js $1 | jq '.' > $2

exit 0
