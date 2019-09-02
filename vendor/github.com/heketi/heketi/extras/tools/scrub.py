#!/usr/bin/python
#
# Copyright (c) 2018 The heketi Authors
#
# This file is licensed to you under your choice of the GNU Lesser
# General Public License, version 3 or any later version (LGPLv3 or
# later), or the GNU General Public License, version 2 (GPLv2), in all
# cases as published by the Free Software Foundation.
#

import argparse
import collections
import json
import logging
import sys

LF = '%(asctime)s: %(levelname)s: %(message)s'


class PendingOperationType(object):
    Unknown = 0
    CreateVolume = 1
    DeleteVolume = 2
    ExpandVolume = 3
    CreateBlockVolume = 4
    DeleteBlockVolume = 5
    RemoveDevice = 6
    CloneVolume = 7

    def __init__(self):
        self.reverse = collections.defaultdict(lambda: 'Unknown', self.rmap())

    @classmethod
    def rmap(cls):
        return {v: k for k, v in cls.__dict__.items() if isinstance(v, int)}


class PendingChangeType(object):
    Unknown = 0
    AddBrick = 1
    AddVolume = 2
    DeleteBrick = 3
    DeleteVolume = 4
    ExpandVolume = 5
    AddBlockVolume = 6
    DeleteBlockVolume = 7
    RemoveDevice = 8
    CloneVolume = 9
    SnapshotVolume = 10
    AddVolumeClone = 11

    def __init__(self):
        self.reverse = collections.defaultdict(lambda: 'Unknown', self.rmap())

    @classmethod
    def rmap(cls):
        return {v: k for k, v in cls.__dict__.items() if isinstance(v, int)}


class EditSet(object):
    def __init__(self):
        self.volumes = set()
        self.bricks = set()
        self.devices = set()
        self.nodes = set()
        self.block_volumes = set()

OP_TYPE = PendingOperationType()
CHANGE_TYPE = PendingChangeType()


def add_to_pending(pi, optype, changetype, uid):
    if changetype == CHANGE_TYPE.AddBrick:
        pi.bricks.add(uid)
    elif changetype == CHANGE_TYPE.AddVolume:
        pi.volumes.add(uid)
    elif changetype == CHANGE_TYPE.DeleteBrick:
        pi.bricks.add(uid)
    elif changetype == CHANGE_TYPE.DeleteVolume:
        pi.volumes.add(uid)
    elif changetype == CHANGE_TYPE.ExpandVolume:
        raise ValueError('expand not supported', optype, changetype)
    elif changetype == CHANGE_TYPE.AddBlockVolume:
        pi.block_volumes.add(uid)
    elif changetype == CHANGE_TYPE.DeleteBlockVolume:
        pi.block_volumes.add(uid)
    elif changetype == CHANGE_TYPE.RemoveDevice:
        pi.devices.add(uid)
    else:
        raise ValueError('not supported', optype, changetype)


def delete_volume(data, vid):
    log.info('deleting volume %s', vid)
    item = data['volumeentries'].pop(vid, None)
    for c in data['clusterentries'].values():
        if vid in c['Info']['volumes']:
            c['Info']['volumes'].remove(vid)
    if not item:
        return
    log.warning('may need manual cleanup: volume: %s',
        item['Info']['name'])


def delete_block_volume(data, vid):
    log.info('deleting block volume %s', vid)
    item = data['blockvolumeentries'].pop(vid, None)
    # remove any reference (real or dangling) from cluster
    for c in data['clusterentries'].values():
        if vid in c['Info']['blockvolumes']:
            c['Info']['blockvolumes'].remove(vid)
    # if no item in db, just stop
    if not item:
        return
    # remove any reference (real or dangling) from BHV
    vol = data['volumeentries'].get(item['Info']['blockhostingvolume'])
    if vol:
        try:
            vol['Info']['blockinfo']['blockvolume'].remove(vid)
        except ValueError:
            log.warning('block volume %s not listed in hosting volume %s',
                        vid, vol['Info']['id'])
    log.warning('may need manual cleanup: block volume: %s',
        item['Info']['blockvolume'].get('iqn') or '???')


def delete_brick(data, bid):
    log.info('deleting brick %s', bid)
    item = data['brickentries'].pop(bid, None)
    for v in data['volumeentries'].values():
        if bid in v['Bricks']:
            v['Bricks'].remove(bid)
    for d in data['deviceentries'].values():
        if bid in d['Bricks']:
            d['Bricks'].remove(bid)
            storage_free(d, item)
    if not item:
        return
    log.warning('may need manual cleanup: brick: %s', item['Info']['path'])


def storage_free(device, brick):
    if not brick:
        return
    total_size = brick['TpSize'] + brick['PoolMetadataSize']
    device['Info']['storage']['free'] += total_size
    device['Info']['storage']['used'] -= total_size
    log.info('added back free size %s to device %s, now: %r',
             total_size, device['Info']['id'],
             device['Info']['storage'])
    if device['Info']['storage']['used'] < 0:
        raise ValueError("used size went negative")
    if device['Info']['storage']['free'] < 0:
        raise ValueError("free size went negative")
    if device['Info']['storage']['free'] > device['Info']['storage']['total']:
        raise ValueError("free size greater than total size")
    if device['Info']['storage']['used'] > device['Info']['storage']['total']:
        raise ValueError("used size greater than total size")


def scrub(data):
    log.debug('starting scrub')
    pending_ops = set()
    p1 = EditSet()
    p2 = EditSet()

    # first pass: scan thru pending entries
    for poid, item in data['pendingoperations'].items():
        pending_ops.add(poid)
        log.info('pending operation: %s', poid)
        optype = item['Type']
        log.info('pending operation type: %s -> %s',
                 optype, OP_TYPE.reverse[optype])
        for a in item['Actions']:
            changetype = a['Change']
            log.info('change type: %s -> %s',
                     changetype, CHANGE_TYPE.reverse[changetype])
            log.info('change type id: %s', a['Id'])
            add_to_pending(p1, optype, changetype, a['Id'])

    # 2nd pass: scan thru items looking for pending
    for vid, item in data['volumeentries'].items():
        poid = item.get('Pending', {}).get('Id')
        if poid:
            p2.volumes.add(vid)

    for vid, item in data['blockvolumeentries'].items():
        poid = item.get('Pending', {}).get('Id')
        if poid:
            p2.block_volumes.add(vid)

    for bid, item in data['brickentries'].items():
        poid = item.get('Pending', {}).get('Id')
        if poid:
            p2.bricks.add(bid)

    for did, item in data['deviceentries'].items():
        poid = item.get('Pending', {}).get('Id')
        if poid:
            p2.devices.add(did)

    # check for items marked as pending not found w/in pending ops
    diffs = 0
    for vid in p2.volumes:
        if vid not in p1.volumes:
            log.warning('volume %s is pending but has no pending op', vid)
            diffs += 1
    for vid in p2.block_volumes:
        if vid not in p1.block_volumes:
            log.warning('block volume %s is pending but has no pending op', vid)
            diffs += 1
    for bid in p2.bricks:
        if bid not in p1.bricks:
            log.warning('brick %s is pending but has no pending op', bid)
            diffs += 1
    for did in p2.devices:
        if did not in p1.devices:
            log.warning('device %s is pending but has no pending op', did)
            diffs += 1
    if diffs:
        raise ValueError("%d differences found -- need manual help" % diffs)

    # cleanup the refrences
    if p1.devices:
        for did in p1.devices:
            log.warning("need manual cleanup: device id: %s", did)
        raise ValueError('need manual cleanup: devices')

    log.debug('scrubbing items')
    for vid in p1.volumes:
        delete_volume(data, vid)
    for vid in p1.block_volumes:
        delete_block_volume(data, vid)
    for bid in p1.bricks:
        delete_brick(data, bid)
    for poid in pending_ops:
        data['pendingoperations'].pop(poid, None)
    return data


def rm_volumes(data, targets):
    if not targets:
        raise ValueError("volume ids to remove must be provided (use -r)")
    log.debug('starting remove volumes: %s', targets)
    es = EditSet()

    # first pass: find out if the vols are real & gather bricks
    log.debug("checking data")
    for volid in targets:
        if volid not in data['volumeentries']:
            raise KeyError('no such volume: {}'.format(volid))
        vol = data['volumeentries'][volid]
        blockvols = vol["Info"].get("blockinfo", {}).get("blockvolume", [])
        if blockvols:
            raise ValueError("volume contains block volumes: {}".format(volid))
        es.volumes.add(volid)
        for b in vol["Bricks"]:
            es.bricks.add(b)

    log.debug('scrubbing items')
    for vid in es.volumes:
        delete_volume(data, vid)
    for bid in es.bricks:
        delete_brick(data, bid)
    return data


def main():
    a = argparse.ArgumentParser()
    a.add_argument('--edit', '-e', default='pending',
                   choices=['pending', 'volumes'])
    a.add_argument('--remove', '-r', action='append', default=[])
    a.add_argument('--logfile', '-l')
    a.add_argument('source')
    cli = a.parse_args()

    if cli.logfile:
        h = logging.FileHandler(cli.logfile)
        h.setFormatter(logging.Formatter(LF))
        logging.getLogger().addHandler(h)

    if cli.source == '-':
        data = json.load(sys.stdin)
    else:
        with open(cli.source) as fh:
            data = json.load(fh)

    if cli.edit == 'pending':
        op = scrub
    elif cli.edit == 'volumes':
        op = lambda d: rm_volumes(data, cli.remove)
    else:
        raise ValueError('unknown edit: {}'.format(cli.edit))

    # use indent=4 in order to match the output of json.tool this
    # is usefule to diff the reformatted original with the update
    json.dump(op(data), sys.stdout, indent=4)
    sys.stdout.write('\n')

logging.basicConfig(
    stream=sys.stderr,
    format=LF,
    level=logging.DEBUG)
log = logging.getLogger('scrub')
main()
