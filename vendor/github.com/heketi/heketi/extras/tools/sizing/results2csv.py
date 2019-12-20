#!/usr/bin/env python
#
# Copyright (c) 2018 The heketi Authors
#
# This file is licensed to you under your choice of the GNU Lesser
# General Public License, version 3 or any later version (LGPLv3 or
# later), or the GNU General Public License, version 2 (GPLv2), in all
# cases as published by the Free Software Foundation.
#

import argparse
import csv
import yaml


def parse_device(d):
    parts = dict(v.split('=') for v in d.split())
    return {k: float(v) if k == 'pct' else int(v) for k, v in parts.items()}


def convert_result(cw, run, results):
    device0 = parse_device(results['device0'])
    device1 = parse_device(results['device1'])
    device2 = parse_device(results['device2'])
    device3 = parse_device(results['device3'])
    deviceTotal = parse_device(results['deviceTotal'])
    cw.writerow([
        run,
        results['total'],
        results['v_mean'],
        results['v_median'],
        # device0
        device0['total'],
        device0['used'],
        device0['pct'],
        # device1
        device1['total'],
        device1['used'],
        device1['pct'],
        # device2
        device2['total'],
        device2['used'],
        device2['pct'],
        # device3
        device3['total'],
        device3['used'],
        device3['pct'],
        # deviceTotal
        deviceTotal['total'],
        deviceTotal['used'],
        deviceTotal['pct'],
    ])


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('source')
    parser.add_argument("dest")
    cli = parser.parse_args()

    with open(cli.dest, 'w') as outfh:
        cw = csv.writer(outfh)
        with open(cli.source) as fh:
            for run, results in enumerate(yaml.load_all(fh)):
                convert_result(cw, run + 1, results)


if __name__ == '__main__':
    main()
