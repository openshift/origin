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
import contextlib
import errno
import json
import logging
import os
import random
import statistics
import subprocess
import time

import heketi


NODE_COUNT = 4

HEKETI_DB = 'heketi.db'
HEKETI_BIN = './heketi'
HEKETI_CONFIG = './heketi.json'
RLOG = 'results_log'

log = logging.getLogger('fitting_room')


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--seed', default=2019, type=int)
    parser.add_argument('--device-size', default=750, type=int)
    parser.add_argument('--target-size', default=1000, type=int)
    parser.add_argument('--min', default=10, type=int)
    parser.add_argument('--max', default=150, type=int)
    parser.add_argument('--max-vols', default=1000, type=int)
    parser.add_argument('--runs', default=1, type=int)
    parser.add_argument('--output', '-o', default=RLOG)
    cli = parser.parse_args()

    for _ in range(cli.runs):
        run(cli)


def run(cli):
    if cli.seed == 0:
        random.seed()
    else:
        random.seed(cli.seed)
    logging.basicConfig(level=logging.DEBUG)
    reset_db()
    os.environ['HEKETI_MOCK_DEVICE_SIZE_GB'] = str(cli.device_size)
    with server():
        setup_cluster()
        plan = generate_plan(
            target=cli.target_size,
            min=cli.min,
            max=cli.max,
            max_vol=cli.max_vols,
        )
        total, record = make_volumes(plan)
    db_json = save_db(0)
    process_results(cli.output, total, record, db_json)


def heketi_client():
    return heketi.HeketiClient('http://localhost:8080', 'x', 'x',
                               poll_delay=0.2)


def reset_db():
    log.info("resetting db")
    try:
        os.unlink(HEKETI_DB)
    except Exception as err:
        log.debug("failed to remove heketi.db: %s", err)
        if err.errno != errno.ENOENT:
            raise


@contextlib.contextmanager
def server():
    log.info('starting server')
    p = subprocess.Popen([HEKETI_BIN, '--config', HEKETI_CONFIG])
    try:
        time.sleep(0.2)
        yield
    finally:
        log.info('stopping server')
        p.kill()
        p.wait()


def setup_cluster(spread_zone=True):
    hc = heketi_client()
    r = hc.cluster_list()
    clusters = r['clusters']
    log.debug('cluster info: %r', clusters)
    if clusters:
        raise ValueError('expected clusters list to be empty')
    r = hc.cluster_create({'block': True, 'file': True})
    cluster_id = r['id']
    for i in range(NODE_COUNT):
        node_req = dict(
            cluster=cluster_id,
            zone=(i+1 if spread_zone else 1),
            hostnames={
                'manage': ['10.1.1.{}'.format(i)],
                'storage': ['100.1.1.{}'.format(i)],
            }
        )
        r = hc.node_add(node_req)
        node_id = r['id']
        device_req = dict(
            node=node_id,
            name='/dev/disk/fake/1',
        )
        ok = hc.device_add(device_req)
        if not ok:
            raise ValueError('device add failed')
    log.debug('vars: %r', vars())


def x_generate_plan():
    m = [10, 20, 50, 89, 75, 100, 100, 150, 250, 500]
    while True:
        yield random.choice(m)


def generate_plan(target=1000, min=10, max=150, max_vol=1000):
    nvols = 0
    t = target
    while True:
        if nvols >= max_vol:
            break
        s = random.randint(min, max)
        if t - s < 0:
            yield t
            break
        yield s
        t = t - s
        nvols += 1


def make_volumes(plan):
    hc = heketi_client()
    log.info('starting volume creation')
    vol_total = 0
    vol_log = []
    for vol_size in plan:
        try:
            r = hc.volume_create(dict(
                size=vol_size,
                durability={'type': 'replicate', 'replicate': {'replica': 3}},
            ))
            log.debug('vol create result: %r', r)
            vol_total += vol_size
            vol_log.append(vol_size)
        except Exception as err:
            log.warning('volume create err: %r', err)
            break
    log.info('created %dG worth of volumes', vol_total)
    return vol_total, vol_log


def save_db(run_id):
    ts = time.strftime('%Y-%m-%d-%H-%M-%S')
    fn = 'heketi-db-{}-{}.json'.format(run_id, ts)
    subprocess.check_call([
        HEKETI_BIN, 'db', 'export',
        '--dbfile', HEKETI_DB,
        '--jsonfile', fn])
    return fn


def process_results(outfn, vol_total, vol_rec, db_json):
    with open(db_json) as fh:
        j = json.load(fh)
    with open(outfn, 'a') as fh:
        fh.write('---\n')
        fh.write('total: {}\n'.format(vol_total))
        fh.write('record: [{}]\n'.format(
            ', '.join(str(v) for v in vol_rec)))
        fh.write('v_mean: {}\n'.format(statistics.mean(vol_rec)))
        fh.write('v_median: {}\n'.format(statistics.median(vol_rec)))
        at = au = 0
        for i, (did, device) in enumerate(j['deviceentries'].items()):
            s = device['Info']['storage']
            t = s['total']
            u = s['used']
            at += t
            au += u
            fh.write('device{}: total={} used={} pct={}\n'.format(
                i, t, u, ((float(u)/t)*100)))
        fh.write('deviceTotal: total={} used={} pct={}\n'.format(
            at, au, ((float(au)/at)*100)))


if __name__ == '__main__':
    main()
