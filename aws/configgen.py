#!/usr/bin/env python3

"""
Broadcast messages in Ethereum transactions.

Usage:
  configgen.py --degree=DEG [options]

Options:
  -h --help                 Show this screen.
  --version                 Show version.
  -d, --degree=DEG          polynomial degree.
  -p, --port=DEG            Port [default: 8000].
"""

from docopt import docopt


def gen_one(degree, port):
    header = """ 
degree = {}

[primary]
url = "{}:{}"

[peers]
"""

    peer_temp = """
[peers.{}]
id={}
url="{}:{}"
"""

    with open("./metadata/addr_list") as ip_input:
        ip_list = list(map(lambda str: str.strip(), ip_input.readlines()))

    with open("scripts/config-deg{}.toml".format(degree), 'w') as out:
        out.write(header.format(degree, ip_list[0], port))
        for i in range(1, 1 + 3 * degree + 1):
            out.write(peer_temp.format(i, i, ip_list[i], port))


def main():
    args = docopt(__doc__)

    degree = int(args['--degree'])
    port = int(args['--port'])


if __name__ == '__main__':
    for degree in (1, 3, 8, 13, 18, 23, 28, 33):
        gen_one(degree, port=8000)
