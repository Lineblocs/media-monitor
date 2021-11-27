#! /bin/bash

if [[ -z "${IP_ADDRESS}" ]]; then
    : ${CLOUD=""} # One of aws, azure, do, gcp, or empty
    if [ "$CLOUD" != "" ]; then
        PROVIDER="-provider ${CLOUD}"
    fi

    PRIVATE_IPV4=$(netdiscover -field privatev4 ${PROVIDER})
    #PRIVATE_IPV4="172.24.0.1"
    PUBLIC_IPV4=$(netdiscover -field publicv4 ${PROVIDER})

    echo "Setting IP to: ${PUBLIC_IPV4}\r\n"
    export IP_ADDRESS=${PUBLIC_IPV4}
fi

./main