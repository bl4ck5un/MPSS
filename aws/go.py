#!/usr/bin/env python3

"""
Usage:
  go.py --config=<cfg>[options]

Options:
  -c, --config=<cfg>        Path to the config file.
  --follow                  Follow the primary log (to see if the protocol finishes).
  --ready                   Check if the log is ready to download.
  --fetch                   Copy logs.
"""

import toml
from docopt import docopt
import subprocess as sp
import os
import sys

KEY_FILE = "mpss.pem"


def ssh(ip, cmd):
    sp.run(["ssh", "-o", "StrictHostKeyChecking=no", "-i", KEY_FILE, "ec2-user@{}".format(ip), cmd])


def check_running(primary_url) -> bool:
    ip = primary_url.split(":")[0]
    cmd = 'docker inspect -f "{{ .State.Running }}" primary'
    p = sp.run(["ssh", "-o", "StrictHostKeyChecking=no", "-i", KEY_FILE, "ec2-user@{}".format(ip), cmd], stdout=sp.PIPE, stderr=sp.PIPE)
    return p.stdout.strip() == b'true'


def follow_log(url, container):
    ip = url.split(":")[0]
    cmd = "docker logs -f {}".format(container)
    ssh(ip, cmd)


def start_primary(degree, url):
    ip = url.split(":")[0]
    cmd = "./scripts/start_primary_docker.sh -c config-deg{}.toml".format(degree)
    ssh(ip, cmd)


def start_node(degree, id, url):
    ip = url.split(":")[0]
    cmd = "./scripts/start_node_docker.sh -c config-deg{}.toml -i {}".format(degree, id)
    ssh(ip, cmd)


def fetch_log(url, remotedir, logdir):
    ip = url.split(":")[0]
    sp.run(["scp", "-i", KEY_FILE, "-r", "ec2-user@{}:/home/ec2-user/scripts/{}/".format(ip, remotedir), logdir])


if __name__ == '__main__':
    args = docopt(__doc__)
    cfg = args["--config"]

    with open(cfg) as t:
        cfg_toml = toml.load(t)

    degree = cfg_toml["degree"]
    primary_url = cfg_toml["primary"]["url"]
    peers = cfg_toml["peers"]

    if args['--follow']:
        follow_log(primary_url, "primary")
    elif args['--fetch']:
        remotedir = "log-{}".format(os.path.basename(cfg))
        logdir = "log"
        sp.run(["mkdir", "-p", logdir])

        fetch_log(primary_url, remotedir, logdir)
        for _, node in peers.items():
            fetch_log(node["url"], remotedir, logdir)
    elif args['--ready']:
        if check_running(primary_url):
            sys.exit(1)
        else:
            sys.exit(0)
    else:
        start_primary(degree, primary_url)

        for _, v in peers.items():
            start_node(degree, v['id'], v['url'])
