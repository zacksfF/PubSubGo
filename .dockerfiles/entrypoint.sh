#!/bin/sh

[ -n "${PUID}" ] && usermod -u "${PUID}" PubSubGo
[ -n "${PGID}" ] && groupmod -g "${PGID}" PubSubGo

printf "Switching UID=%s and GID=%s\n" "${PUID}" "${PGID}"
exec su-exec PubSubGo:PubSubGo "$@"
