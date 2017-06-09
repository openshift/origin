#!/usr/bin/env python

# Copyright 2016 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


#
# Usage:
#
#   $ ./make_graph.py <path to graph file to generate>
#
# This will generate a new graph every 5 seconds in <path>.svg.
#
# Requirements:
#
#   - Must have graphviz (pip install graphviz)
#   - Must have graphviz 'dot' binary in path

import graphviz as gv
import json
import sys
import time
import urllib2

sc_host = 'http://localhost:10000'


def get_nodes():
  """Returns nodes as a dict of instance name -> id."""
  nodes = {}

  idata = urllib2.urlopen(sc_host + '/v2/service_instances').read()
  instances = json.loads(idata)

  if instances:
    for i in instances:
      print('Service Instance: %s, %s' % (i['name'], i['id']))
      nodes[i['name']] = i['id']

  return nodes


def get_edges():
  """Returns edges in the form of ((from_name, to_name), metadata)."""
  edges = []

  bdata = urllib2.urlopen(sc_host + '/v2/service_bindings').read()
  bindings = json.loads(bdata)

  for b in bindings:
    from_name = b['from']
    to_name = b['to']

    print('Binding: %s (%s -> %s)' % (b['name'], from_name, to_name))
    edges.append(((from_name, to_name), {'label':b['name']}))

  return edges


def make_graph(file_name):
  """Generates graph in 'file_name'.svg."""
  g = gv.Digraph(format='svg')

  nodes = get_nodes()

  [ g.node(k) for k,v in nodes.iteritems() ]
  [ g.edge(*e[0], **e[1]) for e in get_edges() ]

  g.render(file_name)


while(1):
  make_graph(sys.argv[1])
  time.sleep(5)
