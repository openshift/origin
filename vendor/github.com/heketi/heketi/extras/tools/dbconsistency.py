#!/usr/bin/python
#
# Copyright (c) 2018 The heketi Authors
#
# This file is licensed to you under your choice of the GNU Lesser
# General Public License, version 3 or any later version (LGPLv3 or
# later), or the GNU General Public License, version 2 (GPLv2), in all
# cases as published by the Free Software Foundation.
#

import collections
import json
import sys


INFO = 'Info'

ERR_COUNT = collections.Counter()


def report(otype, oid, *msg):
    ERR_COUNT[otype] += 1
    print ('{} {}: {}'.format(otype, oid, ': '.join(msg)))


def check_cluster(data, cid, cluster):
    if cid != cluster[INFO]['id']:
        report('Cluster', cid, 'id mismatch', cluster[INFO]['id'])
    for vid in cluster[INFO]['volumes']:
        if vid not in data['volumeentries']:
            report('Cluster', cid, 'unknown volume', vid)
    for vid in cluster[INFO]['blockvolumes']:
        if vid not in data['blockvolumeentries']:
            report('Cluster', cid, 'unknown block volume', vid)


def check_brick(data, bid, brick):
    if bid != brick[INFO]['id']:
        report('Brick', bid, 'id mismatch', brick[INFO]['id'])
    # check brick points to real device
    device_id = brick[INFO]['device']
    if device_id not in data['deviceentries']:
        report('Brick', bid, 'device mismatch', device_id)
    elif bid not in data['deviceentries'][device_id]['Bricks']:
        report('Brick', bid, 'no link back to brick for device', device_id)
    # check that our volume points at the brick
    volume_id = brick[INFO]['volume']
    if volume_id not in data['volumeentries']:
        report('Brick', bid, 'volume mismatch', volume_id)
    elif bid not in data['volumeentries'][volume_id]['Bricks']:
        report('Brick', bid, 'no link back to brick for volume', volume_id)
    # check brick points to real node
    if brick[INFO]['node'] not in data['nodeentries']:
        report('Brick', bid, 'node mismatch', brick[INFO]['node'])
    _check_pending('Brick', bid, brick, data)


def vol_bv_list(volume):
    return volume[INFO].get("blockinfo", {}).get("blockvolume", [])


def check_volume(data, vid, volume):
    if vid != volume[INFO]['id']:
        report('Volume', vid, 'id mismatch', volume[INFO]['id'])
    for bid in volume['Bricks']:
        if bid not in data['brickentries']:
            report('Volume', vid, 'unknown brick', bid)
    bvsize = 0
    for bvid in vol_bv_list(volume):
        if bvid not in data["blockvolumeentries"]:
            report('Volume', vid, 'unknown block volume', bvid)
        else:
            bv = data["blockvolumeentries"][bvid]
            bvsize += bv[INFO]['size']
    if volume[INFO].get("block"):
        vol_size = volume[INFO]["size"]
        free_size = volume[INFO].get("blockinfo", {}).get("freesize", 0)
        rsvd_size = volume[INFO].get("blockinfo", {}).get("reservedsize", 0)
        used_size = vol_size - free_size - rsvd_size
        if bvsize != used_size:
            rf = ('block-vol-sum={}'
                  ' size={} free-size={} reserved-size={} used-size={}')
            report(
                'Volume', vid, 'block size differs',
                rf.format(bvsize, vol_size, free_size, rsvd_size, used_size))
    elif bvsize != 0:
        report('Volume', vid, 'has block volumes but not block flag')
    _check_pending('Volume', vid, volume, data)


def check_block_volume(data, bvid, bvol):
    if bvid != bvol[INFO]['id']:
        report('Block Volume', bvid, 'id mismatch', bvol[INFO]['id'])
    cluster_id = bvol[INFO].get("cluster")
    bhv_id = bvol[INFO]["blockhostingvolume"]
    if cluster_id and cluster_id not in data["clusterentries"]:
        report("Block Volume", bvid, "cluster not found", cluster_id)
    if bhv_id not in data["volumeentries"]:
        report("Block Volume", bvid, "hosting volume not found", bhv_id)
    elif bvid not in vol_bv_list(data["volumeentries"][bhv_id]):
        report("Block Volume", bvid, "not tracked in hosting volume", bhv_id)
    _check_pending('Block Volume', bvid, bvol, data)


def check_device(data, did, device):
    if did != device[INFO]['id']:
        report('Device', did, 'id mismatch', device[INFO]['id'])
    bsum = 0
    for bid in device['Bricks']:
        if bid not in data['brickentries']:
            report('Device', did, 'unknown brick', bid)
        else:
            b = data['brickentries'][bid]
            bsum += b["TpSize"] + b["PoolMetadataSize"]
    s = device[INFO]["storage"]
    if s["total"] != s["free"] + s["used"]:
        report("Device", did, "size values differ",
               "total={total}  free={free} used={used}".format(**s))
    if s["used"] != bsum:
        report("Device", did, "size values differ",
               "used={} brick-sum={}".format(s["used"], bsum))
    _check_pending('Device', did, device, data)


def check_pending(data, pid, pop):
    if pid != pop["Id"]:
        report("Pending Op", pid, "id mismatch", pop["Id"])
    changetype_to_key = {
        1: "brickentries",  # add brick
        2: "volumeentries",  # add vol
        3: "brickentries",  # del brick
        4: "volumeentries",  # del vol
        5: "volumeentries",  # ExpandVolume
        6: "blockvolumeentries",  # AddBlockVolume
        7: "blockvolumeentries",  # DeleteBlockVolume
        8: "deviceentries",  # RemoveDevice
        9: "volumeentries",  # CloneVolume
        10: "volumeentries",  # SnapshotVolume
        11: "volumeentries",  # AddVolumeClone
    }
    for a in pop["Actions"]:
        ch_type = a["Change"]
        ch_id = a["Id"]
        key = changetype_to_key.get(ch_type)
        if not key:
            report("Pending Op", pid, "unexpected change type", ch_type)
            continue
        if ch_id not in data[key]:
            report("Pending Op", pid, "id in change missing",
                   "{} not found in {}".format(ch_id, key))


def _check_pending(what, myid, item, data):
    pid = item.get("Pending", {}).get("Id")
    if not pid:
        return
    if pid not in data["pendingoperations"]:
        report(what, myid, "marked pending but no pending op", pid)


def check_db(data):
    for cid, c in data['clusterentries'].items():
        check_cluster(data, cid, c)

    for vid, v in data['volumeentries'].items():
        check_volume(data, vid, v)

    for bvid, bv in data['blockvolumeentries'].items():
        check_block_volume(data, bvid, bv)

    for did, d in data['deviceentries'].items():
        check_device(data, did, d)

    for bid, b in data['brickentries'].items():
        check_brick(data, bid, b)

    for pid, p in data['pendingoperations'].items():
        check_pending(data, pid, p)


def summarize_db(data):
    cc = collections.Counter()
    kc = [
        ('clusters', 'clusterentries'),
        ('devices', 'deviceentries'),
        ('bricks', 'brickentries'),
        ('volumes', 'volumeentries'),
        ('blockvolumes', 'blockvolumeentries'),
        ('pending', 'pendingoperations'),
    ]
    for key, data_key in kc:
        cc[key] = len(data[data_key])
        pc = len([x for x in data[data_key].values()
                  if x.get('Pending', {}).get('Id')])
        if pc:
            cc['pending_' + key] = pc

    for key, _ in kc:
        if key.startswith('pending_'):
            continue
        pk = 'pending_' + key
        if pk in cc:
            print ('{:>16}: {:5d}  ({:d} pending)'.format(
                key.title(), cc[key], cc[pk]))
        else:
            print ('{:>16}: {:5d}'.format(key.title(), cc[key]))
    print ('')


try:
    filename = sys.argv[1]
except IndexError:
    sys.stderr.write("error: filename required\n")
    sys.exit(2)

with open(filename) as fh:
    data = json.load(fh)

summarize_db(data)
check_db(data)
if ERR_COUNT:
    sys.exit(1)
