#
# Copyright (c) 2015 The heketi Authors
#
# This file is licensed to you under your choice of the GNU Lesser
# General Public License, version 3 or any later version (LGPLv3 or
# later), as published by the Free Software Foundation,
# or under the Apache License, Version 2.0 <LICENSE-APACHE2 or
# http://www.apache.org/licenses/LICENSE-2.0>.
#
# You may not use this file except in compliance with those terms.
#
#
# Usage:
# # from heketi import HeketiClient
# # client = HeketiClient(server, user, key)
# # Eg.: Cluster creation: client.cluster_create()

import jwt
import datetime
import hashlib
import requests
import time
import json


# Constants related to setting tags
TAGS_SET = 'set'
TAGS_UPDATE = 'update'
TAGS_DELETE = 'delete'


class HeketiClient(object):

    def __init__(self, host, user, key, verify=True):
        self.host = host
        self.user = user
        self.key = key
        self.verify = verify

    def _set_token_in_header(self, method, uri, headers={}):
        claims = {}
        claims['iss'] = self.user

        # Issued at time
        claims['iat'] = datetime.datetime.utcnow()

        # Expiration time
        claims['exp'] = datetime.datetime.utcnow() \
            + datetime.timedelta(seconds=1)

        # URI tampering protection
        val = b'%s&%s' % (method.encode('utf8'), uri.encode('utf8'))
        claims['qsh'] = hashlib.sha256(val).hexdigest()

        token = jwt.encode(claims, self.key, algorithm='HS256')
        headers['Authorization'] = b'bearer ' + token

        return headers

    def hello(self):
        method = 'GET'
        uri = '/hello'

        headers = self._set_token_in_header(method, uri)
        r = requests.get(self.host + uri, headers=headers, verify=self.verify)
        return r.status_code == requests.codes.ok

    def _make_request(self, method, uri, data={}, headers={}):
        '''
        Ref:
        http://docs.python-requests.org
              /en/master/_modules/requests/api/#request
        '''
        headers.update(self._set_token_in_header(method, uri))
        r = requests.request(method,
                             self.host + uri,
                             headers=headers,
                             data=json.dumps(data),
                             verify=self.verify)

        r.raise_for_status()

        if r.status_code == requests.codes.accepted:
            return self._get_queued_response(r.headers['location'])
        else:
            return r

    def _get_queued_response(self, queue_uri):
        queue_uri = queue_uri
        response_ready = False

        while response_ready is False:
            headers = self._set_token_in_header('GET', queue_uri)
            q = requests.get(self.host + queue_uri,
                             headers=headers,
                             allow_redirects=False,
                             verify=self.verify)

            # Raise an exception when the request fails
            q.raise_for_status()

            if 'X-Pending' in q.headers:
                time.sleep(2)
            else:
                if q.status_code == requests.codes.see_other:
                    return self._make_request('GET', q.headers['location'])
                else:
                    return q

    def cluster_create(self, cluster_options={}):
        ''' cluster_options is a dict with cluster creation options:
            https://github.com/heketi/heketi/wiki/API#cluster_create
        '''
        req = self._make_request('POST', '/clusters', cluster_options)
        if req.status_code == requests.codes.created:
            return req.json()

    def cluster_setflags(self, cluster_id, cluster_options={}):
        uri = "/clusters/" + cluster_id + "/flags"
        req = self._make_request('POST', uri, cluster_options)
        return req.status_code == requests.codes.ok

    def cluster_info(self, cluster_id):
        uri = "/clusters/" + cluster_id
        req = self._make_request('GET', uri)
        if req.status_code == requests.codes.ok:
            return req.json()

    def cluster_list(self):
        uri = "/clusters"
        req = self._make_request('GET', uri)
        if req.status_code == requests.codes.ok:
            return req.json()

    def cluster_delete(self, cluster_id):
        uri = "/clusters/" + cluster_id
        req = self._make_request('DELETE', uri)
        return req.status_code == requests.codes.ok

    def node_add(self, node_options={}):
        '''
        node_options is a dict consisting of paramters for
        adding a node: https://github.com/heketi/heketi/wiki/API#add-node
        '''
        uri = "/nodes"
        req = self._make_request('POST', uri, node_options)
        if req.status_code == requests.codes.ok:
            return req.json()

    def node_info(self, node_id):
        uri = '/nodes/' + node_id
        req = self._make_request('GET', uri)
        if req.status_code == requests.codes.ok:
            return req.json()

    def node_delete(self, node_id):
        uri = '/nodes/' + node_id
        req = self._make_request('DELETE', uri)
        return req.status_code == requests.codes.NO_CONTENT

    def node_state(self, node_id, state_request={}):
        uri = '/nodes/' + node_id + '/state'
        req = self._make_request('POST', uri, state_request)
        return req.status_code == requests.codes.NO_CONTENT
        '''if req.status_code == requests.codes.ok:
            return req.json()
        return req.status_code == requests.codes.ok
        '''

    def node_set_tags(self, node_id, tags_options):
        '''Set, update, or delete tags on a node.
        Specify tags with options key "tags" and a dict of tags & values,
        specify change type with options key "change_type" and a
        string value of "set", "update", "delete" (or use TAGS_* constants).
        '''
        uri = '/nodes/' + node_id + '/tags'
        req = self._make_request('POST', uri, tags_options)
        return req.status_code == requests.codes.ok

    def device_add(self, device_options={}):
        ''' device_options is a dict with parameters to be passed \
            in the json request: \
            https://github.com/heketi/heketi/wiki/API#add-device
        '''
        uri = '/devices'
        req = self._make_request('POST', uri, device_options)
        return req.status_code == requests.codes.NO_CONTENT

    def device_info(self, device_id):
        uri = '/devices/' + device_id
        req = self._make_request('GET', uri)
        if req.status_code == requests.codes.ok:
            return req.json()

    def device_delete(self, device_id):
        uri = '/devices/' + device_id
        req = self._make_request('DELETE', uri)
        return req.status_code == requests.codes.NO_CONTENT

    def device_state(self, device_id, state_request={}):
        uri = "/devices/" + device_id + "/state"
        req = self._make_request('POST', uri, state_request)
        return req.status_code == requests.codes.NO_CONTENT
        '''if req.status_code == requests.codes.ok:
            return req.json()
        return req.status_code == requests.codes.ok
        '''

    def device_resync(self, device_id):
        uri = '/devices/' + device_id + '/resync'
        req = self._make_request('GET', uri)
        return req.status_code == requests.codes.NO_CONTENT

    def device_set_tags(self, device_id, tags_options):
        '''Set, update, or delete tags on a device.
        Specify tags with options key "tags" and a dict of tags & values,
        specify change type with options key "change_type" and a
        string value of "set", "update", "delete" (or use TAGS_* constants).
        '''
        uri = '/devices/' + device_id + '/tags'
        req = self._make_request('POST', uri, tags_options)
        return req.status_code == requests.codes.ok

    def volume_create(self, volume_options={}):
        ''' volume_options is a dict with volume creation options:
            https://github.com/heketi/heketi/wiki/API#create-a-volume
        '''
        uri = '/volumes'
        req = self._make_request('POST', uri, volume_options)
        if req.status_code == requests.codes.ok:
            return req.json()

    def volume_list(self):
        uri = '/volumes'
        req = self._make_request('GET', uri)
        if req.status_code == requests.codes.ok:
            return req.json()

    def volume_info(self, volume_id):
        uri = '/volumes/' + volume_id
        req = self._make_request('GET', uri)
        if req.status_code == requests.codes.ok:
            return req.json()

    def volume_expand(self, volume_id, expand_size={}):
        uri = '/volumes/' + volume_id + '/expand'
        req = self._make_request('POST', uri, expand_size)
        if req.status_code == requests.codes.ok:
            return req.json()

    def volume_delete(self, volume_id):
        uri = '/volumes/' + volume_id
        req = self._make_request('DELETE', uri)
        return req.status_code == requests.codes.NO_CONTENT

    def block_volume_list(self):
        uri = '/blockvolumes'
        req = self._make_request('GET', uri)
        if req.status_code == requests.codes.ok:
            return req.json()

    def block_volume_info(self, volume_id):
        uri = '/blockvolumes/' + volume_id
        req = self._make_request('GET', uri)
        if req.status_code == requests.codes.ok:
            return req.json()

    def block_volume_create(self, volume_options={}):
        uri = '/blockvolumes'
        req = self._make_request('POST', uri, volume_options)
        if req.status_code == requests.codes.ok:
            return req.json()

    def block_volume_delete(self, volume_id):
        uri = '/blockvolumes/' + volume_id
        req = self._make_request('DELETE', uri)
        return req.status_code == requests.codes.NO_CONTENT

    def db_dump(self):
        uri = '/db/dump'
        req = self._make_request('GET', uri)
        if req.status_code == requests.codes.ok:
            return req
