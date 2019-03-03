#!/usr/bin/env python3

import os
import json
from typing import List
from matplotlib import pyplot as plt
import math
import numpy as np
import pandas as pd

ROOTDIR="log"
BENCHMARK_FILENAME="1-benchmark.log"

class Entry:
    def __init__(self, deg, group_size, latency, latency_std, offchain, offchain_std, onchain, onchain_std):
        self.degree = deg
        self.group_size = group_size
        self.latency = latency
        self.latency_std = latency_std
        self.offchain = offchain
        self.offchain_std = offchain_std
        self.onchain = onchain
        self.onchain_std = onchain_std


class Result:
    entries: List[Entry] = []

    def add(self, e: Entry):
        self.entries.append(e)

    def _sort(self):
        self.entries = sorted(self.entries, key=lambda e: e.degree)

    def _by_groupsize(self, item, ylabel, figname) -> pd.DataFrame:
        self._sort()

        gsz = []
        lat = []
        lat_err = []
        for e in self.entries:
            gsz.append(e.group_size)
            lat.append(e.__getattribute__(item))
            lat_err.append(e.__getattribute__("{}_std".format(item)))

        plt.figure()
        plt.errorbar(gsz, lat, yerr=lat_err)
        plt.xlabel("committee size")
        plt.ylabel(ylabel)
        plt.savefig(figname)

        d = pd.DataFrame()
        d['x'] = gsz
        d['y'] = lat
        d['y_err'] = lat_err

        return d

    def latency_x_groupsize(self):
        d = self._by_groupsize("latency", "latency (second)", "latency.png")
        d.to_csv("latency.dat", sep='\t', index=False)

    def onchain_x_groupsize(self):
        d = self._by_groupsize("onchain", "On-chain message complexity (bytes per epoch)", "onchain.png")
        d.to_csv("onchain.dat", sep='\t', index=False)

    def offchain_x_groupsize(self):
        self._sort()

        x = []
        y = []
        y_err = []
        for e in self.entries:
            x.append(e.group_size)
            y.append(e.offchain * e.group_size)
            y_err.append(e.offchain_std * math.sqrt(e.group_size))

        y = np.array(y) / 1e6
        y_err = np.array(y) / 1e6

        plt.figure()
        plt.errorbar(x, y, yerr=y_err)
        plt.xlabel("committee size")
        plt.ylabel("Off-chain message complexity (MB per epoch)")
        plt.savefig("offchain.png")

        d = pd.DataFrame()
        d['x'] = x
        d['y'] = y
        d['y_err'] = y_err
        d.to_csv("offchain.dat", sep='\t', index=False)


def parse_log_dir(logdir) -> Entry:
    with open(os.path.join(logdir, BENCHMARK_FILENAME)) as json_f:
        benchmark_obj = json.load(json_f)

        keys = ("degree", "groupsize", "latencyMean", "latencyStd", "offChainMean", "offChainStd", "onChainMean", "onChainStd")
        values = tuple(map(lambda k: benchmark_obj[k], keys))

        return Entry(*values)


def main():
    r = Result()

    log_dirs = os.listdir(ROOTDIR)

    for d in log_dirs:
        if os.path.isfile(d):
            continue
        print(d)
        r.add(parse_log_dir(os.path.join(ROOTDIR, d)))

    r.latency_x_groupsize()
    r.onchain_x_groupsize()
    r.offchain_x_groupsize()

if __name__ == '__main__':
    main()
