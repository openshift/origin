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

EXAMPLE = """
Example:
   $ python3 comparison.py
        --gluster-info gluster-volume-info.txt
        --heketi-json heketi-db.json
        --pv-yaml openshift-pv-yaml.yaml
"""

# flag constants
IN_GLUSTER = 'gluster'
IN_HEKETI = 'heketi'
IN_PVS = 'pvs'
IS_BLOCK = 'BV'


class CliError(ValueError):
    pass


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
    parser.add_argument(
        '--match-storage-class', '-S', action='append',
        help='Match one or more storage class names')
    parser.add_argument(
        '--skip-block', action='store_true',
        help='Exclude block volumes from output')
    parser.add_argument(
        '--bricks', action='store_true',
        help='Compare bricks rather than volumes')

    cli = parser.parse_args()
    try:
        if cli.bricks:
            return examine_bricks(cli)
        return examine_volumes(cli)
    except CliError as err:
        parser.error(str(err))


def examine_volumes(cli):
    check = []
    gvinfo = heketi = pvdata = None
    if cli.gluster_info:
        check.append(IN_GLUSTER)
        gvinfo = parse_gvinfo(cli.gluster_info)
    if cli.heketi_json:
        check.append(IN_HEKETI)
        heketi = parse_heketi(cli.heketi_json)
    if cli.pv_yaml:
        check.append(IN_PVS)
        pvdata = parse_oshift(cli.pv_yaml)

    if not check:
        raise CliError(
            "Must provide: --gluster-info OR --heketi-json OR --pv-yaml")

    summary = compile_summary(cli, gvinfo, heketi, pvdata)
    for ign in (cli.ignore or []):
        if summary.pop(ign, None):
            sys.stderr.write('ignoring: {}\n'.format(ign))
    compare(summary, check, cli.skip_ok,
            header=(not cli.no_header),
            show_pending=(cli.pending),
            skip_block=cli.skip_block)
    return


def examine_bricks(cli):
    check = []
    gvinfo = heketi = None
    if cli.gluster_info:
        check.append(IN_GLUSTER)
        gvinfo = parse_gvinfo(cli.gluster_info)
    if cli.heketi_json:
        check.append(IN_HEKETI)
        heketi = parse_heketi(cli.heketi_json)

    if not check:
        raise CliError(
            "Must provide: --gluster-info and --heketi-json")

    summary = compile_brick_summary(cli, gvinfo, heketi)
    compare_bricks(summary, check,
                   skip_ok=cli.skip_ok)


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
        summary[n] = {'id': vid, IN_HEKETI: True}
        if v['Pending']['Id']:
            summary[n]['heketi-pending'] = True
        if v['Info'].get('block'):
            summary[n]['heketi-bhv'] = True
    for bvid, bv in heketi['blockvolumeentries'].items():
        n = bv['Info']['name']
        summary[n] = {
            IN_HEKETI: True,
            'block': True,
            'id': bvid,
        }
        if bv['Pending']['Id']:
            summary[n]['heketi-pending'] = True


def compile_heketi_bricks(summary, heketi):
    for bid, b in heketi['brickentries'].items():
        path = b['Info']['path']
        node_id = b['Info']['node']
        vol_id = b['Info']['volume']
        host = (heketi['nodeentries'][node_id]
                      ['Info']['hostnames']['storage'][0])
        vol_name = heketi['volumeentries'][vol_id]['Info']['name']
        fbp = '{}:{}'.format(host, path)
        dest = summary.setdefault(fbp, {})
        dest[IN_HEKETI] = True
        dest['heketi_volume'] = vol_name


def compile_gvinfo(summary, gvinfo):
    for vn in gvinfo:
        if vn in summary:
            summary[vn][IN_GLUSTER] = True
        else:
            summary[vn] = {IN_GLUSTER: True}


def compile_gvinfo_bricks(summary, gvinfo):
    for vn, content in gvinfo.items():
        for bn in content:
            dest = summary.setdefault(bn, {})
            dest[IN_GLUSTER] = True
            dest['gluster_volume'] = vn


def compile_pvdata(summary, pvdata, matchsc):
    for elem in pvdata['items']:
        g = elem.get('spec', {}).get('glusterfs', {})
        ma = elem.get('metadata', {}).get('annotations', {})
        if not g and 'glusterBlockShare' not in ma:
            continue
        sc = elem.get('spec', {}).get('storageClassName', '')
        if matchsc and sc not in matchsc:
            sys.stderr.write(
                'ignoring: {} from storage class "{}"\n'.format(g["path"], sc))
            continue
        if 'path' in g:
            vn = g['path']
            block = False
        elif 'glusterBlockShare' in ma:
            vn = ma['glusterBlockShare']
            block = True
        else:
            raise KeyError('path (volume name) not found in PV data')
        dest = summary.setdefault(vn, {})
        dest[IN_PVS] = True
        if block:
            dest['block'] = True


def compile_summary(cli, gvinfo, heketi, pvdata):
    summary = {}
    if heketi:
        compile_heketi(summary, heketi)
    if gvinfo:
        compile_gvinfo(summary, gvinfo)
    if pvdata:
        compile_pvdata(summary, pvdata, matchsc=cli.match_storage_class)
    return summary


def compile_brick_summary(cli, gvinfo, heketi):
    summary = {}
    if gvinfo:
        compile_gvinfo_bricks(summary, gvinfo)
    if heketi:
        compile_heketi_bricks(summary, heketi)
    return summary


def _check_item(vname, vstate, check):
    tocheck = set(check)
    flags = []
    if vstate.get('block'):
        flags.append(IS_BLOCK)
        # block volumes will never be found in gluster info
        tocheck.discard(IN_GLUSTER)
    m = set(c for c in tocheck if vstate.get(c))
    flags.extend(sorted(m))
    return m == tocheck, flags


def compare(summary, check, skip_ok=False, header=True, show_pending=False,
            skip_block=False):
    if header:
        _print = Printer(['Volume-Name', 'Match', 'Volume-ID'])
    else:
        _print = Printer([])

    for vn, vs in summary.items():
        ok, flags = _check_item(vn, vs, check)
        if ok and skip_ok:
            continue
        if 'BV' in flags and skip_block:
            continue
        heketi_info = vs.get('id', '')
        if show_pending and vs.get('heketi-pending'):
            heketi_info += '/pending'
        if vs.get('heketi-bhv'):
            heketi_info += '/block-hosting'
        if ok:
            sts = 'ok'
        else:
            sts = ','.join(flags)
        _print.line(vn, sts, heketi_info)


def _check_brick(bpath, bstate, check):
    tocheck = set(check)
    flags = []
    volumes = []
    m = set(c for c in tocheck if bstate.get(c))
    flags.extend(sorted(m))
    gv = bstate.get('gluster_volume')
    hv = bstate.get('heketi_volume')
    ok = False
    if m == tocheck and gv == hv:
        ok = True
        volumes = ['match={}'.format(gv)]
    else:
        if gv:
            volumes.append('gluster={}'.format(gv))
        if hv:
            volumes.append('heketi={}'.format(hv))
    return ok, flags, volumes


def compare_bricks(summary, check, header=True, skip_ok=False):
    if header:
        _print = Printer(['Brick-Path', 'Match', 'Volumes'])
    else:
        _print = Printer([])

    for bp, bstate in summary.items():
        ok, flags, volumes = _check_brick(bp, bstate, check)
        if ok and skip_ok:
            continue
        if ok:
            sts = 'ok'
        else:
            sts = ','.join(flags)
        _print.line(bp, sts, ','.join(volumes))


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
