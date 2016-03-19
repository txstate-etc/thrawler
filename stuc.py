#!/usr/bin/python3
import sys
import json

for line in sys.stdin:
  data = json.loads(line)
  print(data["src"] + "\t" + data["tag"] + "\t" + data["url"] + "\t" + str(data["code"]))
