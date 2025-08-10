tar cf - $(find . -name '*.go') | ssh root@134.209.43.127 "cd /opt/analytics && tar xf -"
scp go.mod root@134.209.43.127:/opt/analytics/
# scp ./bin/*.json root@134.209.43.127:/opt/analytics/bin/