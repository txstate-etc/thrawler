#!/usr/bin/python3
# nodes.py is to recursively gather page links
# from magnolia RESTful output of nodes.
# Example usage:
# curl --user '...' -H 'Accept: application/json' 'http://localhost:8080/mjdf38i3tv0b56vz/.rest/nodes/v1/website/testing-site-destroyer?depth=999&excludeNodeTypes=mgnl:resource,mgnl:metaData,mgnl:content,mgnl:contentNode,mgnl:area,mgnl:component,mgnl:nodeData,mgnl:user,mgnl:group,mgnl:role'| ./nodes.py
import sys
import json

def nodes(d):
  for k, v in d.items():
    if k == "path":
      print(v)
    elif k == "nodes":
      for n in v:
        nodes(n)

for line in sys.stdin:
  nodes(json.loads(line))
