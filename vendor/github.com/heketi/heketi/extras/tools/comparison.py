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
import json
import sys
import yaml


DESC = """
Compare outputs of gluster and/or heketi and/or openshift/k8s.
Prints lists of volumes where sources differ.
"""

EXAMPLE= """
Example:
   $ python3 comparison.py
        --gluster-info gluster-volume-info.txt
        --heketi-json heketi-db.json
        --pv-yaml openshift-pv-yaml.yaml
"""

def main():
    parser = argparse.ArgumentParser(description=DESC, epilog=EXAMPLE)
    parser.add_argument(
        '--gluster-info', '-g',
        help='Path to a file containing gluster volume info')
    parser.add_argument(
        '--heketi-json', '-j',
        help='Path to a file containing Heketi db json export')
    parser.add_argument(
        '--pv-yaml', '-y',
        help='Path to a file containing PV yaml data')
    parser.add_argument(
        '--skip-ok', '-K', action='store_true',
        help='Exclude matching items from output')
    parser.add_argument(
        '--pending', action='store_true',
        help='Show heketi pending status (best effort)')
    parser.add_argument(
        '--no-header', '-H', action='store_true',
        help='Do not print column header')
    parser.add_argument(
        '--ignore', '-I', action='append',
        help='Exlude given volume name (multiple allowed)')

    cli = parser.parse_args()

    check = []
    gvinfo = heketi = pvdata = None
    if cli.gluster_info:
        check.append('gluster')
        gvinfo = parse_gvinfo(cli.gluster_info)
    if cli.heketi_json:
        check.append('heketi')
        heketi = parse_heketi(cli.heketi_json)
    if cli.pv_yaml:
        check.append('pvs')
        pvdata = parse_oshift(cli.pv_yaml)

    if not check:
        parser.error(
            "Must provide: --gluster-info OR --heketi-json OR --pv-yaml")

    summary = compile_summary(gvinfo, heketi, pvdata)
    for ign in (cli.ignore or []):
        if summary.pop(ign, None):
            sys.stderr.write('ignoring: {}\n'.format(ign))
    compare(summary, check, cli.skip_ok,
            header=(not cli.no_header),
            show_pending=(cli.pending))
    return


def parse_heketi(h_json):
    with open(h_json) as fh:
        return json.load(fh)


def parse_oshift(yf):
    with open(yf) as fh:
        return yaml.safe_load(fh)


def parse_gvlist(gvl):
    vols = {}
    with open(gvl) as fh:
        for line in fh:
            vols[line.strip()] = []
    return vols


def parse_gvinfo(gvi):
    vols = {}
    volume = None
    with open(gvi) as fh:
        for line in fh:
            l = line.strip()
            if l.startswith("Volume Name:"):
                volume = l.split(":", 1)[-1].strip()
                vols[volume] = []
            if l.startswith('Brick') and l != "Bricks:":
                if volume is None:
                    raise ValueError("Got Brick before volume: %s" % l)
                vols[volume].append(l.split(":", 1)[-1].strip())
    return vols


def compile_heketi(summary, heketi):
    for vid, v in heketi['volumeentries'].items():
        n = v['Info']['name']
        summary[n] = {'id': vid, 'heketi': True}
        if v['Pending']['Id']:
            summary[n]['heketi-pending'] = True


def compile_gvinfo(summary, gvinfo):
    for vn in gvinfo:
        if vn in summary:
            summary[vn]['gluster'] = True
        else:
            summary[vn] = {'gluster': True}


def compile_pvdata(summary, pvdata):
    for elem in pvdata['items']:
        g = elem.get('spec', {}).get('glusterfs', {})
        if not g:
            continue
        vn = g['path']
        if vn in summary:
            summary[vn]['pvs'] = True
        else:
            summary[vn] = {'pvs': True}


def compile_summary(gvinfo, heketi, pvdata):
    summary = {}
    if heketi:
        compile_heketi(summary, heketi)
    if gvinfo:
        compile_gvinfo(summary, gvinfo)
    if pvdata:
        compile_pvdata(summary, pvdata)
    return summary


def compare(summary, check, skip_ok=False, header=True, show_pending=False):
    if header:
        _print = Printer(['Volume-Name', 'Match', 'Volume-ID'])
    else:
        _print = Printer([])

    for vn, vs in summary.items():
        ok = all(vs.get(c) for c in check)
        if ok and skip_ok:
            continue
        heketi_info = vs.get('id', '')
        if show_pending and vs.get('heketi-pending'):
            heketi_info += '/pending'
        if ok:
            _print.line(vn, 'ok', heketi_info)
        else:
            matches = ','.join(
                sorted(k for k in check if vs.get(k)))
            _print.line(vn, matches, heketi_info)


class Printer(object):
    """Utility class for printing columns w/ headers."""
    def __init__(self, header):
        self._did_header = False
        self.header = header or []

    def line(self, *columns):
        if self.header and not self._did_header:
            self._print_header(columns)
            self._did_header = True
        print (' '.join(columns))

    def _print_header(self, columns):
        parts = []
        for idx, hdr in enumerate(self.header):
            pad = max(0, len(columns[idx]) - len(hdr))
            parts.append('{}{}'.format(hdr, ' ' * pad))
        print (' '.join(parts))


if __name__ == '__main__':
    main()
