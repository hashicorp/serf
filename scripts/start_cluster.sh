#!/bin/bash
SERF="./bin/serf"
SECRET="1FzgH8LsTtr0Wopn4934OQ=="
for i in {0..99}
do
    BIND=`expr 2 + $i`
    PORT=`expr 7373 + $i`
    echo Starting Serf agent $i on 127.0.0.1:$BIND, RPC on port $PORT
    $SERF agent -node=node$i -rpc-addr=127.0.0.1:$PORT -bind=127.0.0.$BIND -log-level=warn -encrypt=$SECRET -join=127.0.0.2 &
    if [ $i -eq 0 ]
    then
        sleep 1;
    fi
done
