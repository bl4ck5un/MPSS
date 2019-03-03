#!/usr/bin/env bash

set -e

ROOTDIR=$( cd "$( dirname "${BASH_SOURCE[0]}")" && pwd )

round=10

while getopts ":c:i:" opt; do
  case ${opt} in
    i)
      id=$OPTARG
      ;;
    c)
      config=$OPTARG
      ;;
    \?)
      echo "Invalid option: $OPTARG" 1>&2
      exit -1
      ;;
    :)
      echo "Invalid option: $OPTARG requires an argument" 1>&2
      exit -1
      ;;
  esac
done
shift $((OPTIND -1))

cd ${ROOTDIR}

test -f ${config} || {
    echo "$1 is not a config file"
    exit -1
}

test -z ${config} && {
    echo "please provide a config file with -c CONFIG"
    exit -1
}

test -z ${id} && {
    echo "please provide an id with -i ID"
    exit -1
}

docker run --rm --name "node" \
    -d \
    -v $(pwd)/log-${config}/:/log \
    -v $(pwd)/${config}:/config \
    -p 8000:8000 \
    churp/mpss /node --id ${id} --config /config --debug --logdir=/log --round $round