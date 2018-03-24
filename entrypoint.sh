#!/usr/bin/env sh

addgroup -g $USER_GID user
adduser -h /home/user -G user -D -u $USER_UID user

su-exec user /usr/local/bin/loghouse-acceptor &
child=$!

trap "kill $child" SIGTERM SIGINT
wait "$child"
trap - SIGTERM SIGINT
wait "$child"
