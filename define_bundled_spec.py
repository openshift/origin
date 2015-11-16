#!/usr/bin/env python

import json

dlist = json.load(open('Godeps/Godeps.json'))

for bdep in [(dep[u'ImportPath'], dep[u'Rev']) for dep in dlist[u'Deps']]:
    print ("Provides: golang(bundled({0})) = {1}".format(bdep[0], bdep[1]))
