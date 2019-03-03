#!/usr/bin/env bash

ROOTDIR=$( cd "$( dirname "${BASH_SOURCE[0]}")" && pwd )

while getopts ":c:" opt; do
  case ${opt} in
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

docker run --rm --name "primary" \
    -d \
    -v $(pwd)/log-${config}/:/log \
    -v $(pwd)/${config}:/config \
    -p 8000:8000 \
    churp/mpss /primary --config /config --logdir=/log --debug