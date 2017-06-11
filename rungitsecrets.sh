#!/bin/bash

cd $1

git secrets --install
git secrets --register-aws
git secrets --add 'xoxp-.*'
git secrets --add 'xoxb-.*'
git secrets --scan -r . > $2

exit 0