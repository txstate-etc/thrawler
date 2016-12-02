#!/bin/bash
. .env

name="${1:-before}"
# Setup for sequential after passes
if [ "$name" == 'before' ]; then
  rm -f after.miss.txt after.link.txt
  >$name.miss.txt
elif [ "$name" == 'after' ]; then
  if [ -f "after.miss.txt" ]; then
    mv after.miss.txt before.miss.txt
  fi
  if [ -f "after.link.txt" ]; then
    mv after.link.txt before.link.txt
  fi
else
  echo "USAGE: $0 [before|after]"
  exit 1
fi

# Tee off json for thrawler debugging.
# Tee off node info for magnolia RESTful
# debugging.
curl --user "$magusr" -H 'Accept: application/json' 'http://localhost:8080/mjdf38i3tv0b56vz/.rest/nodes/v1/website/testing-site-destroyer?depth=999&excludeNodeTypes=mgnl:resource,mgnl:metaData,mgnl:content,mgnl:contentNode,mgnl:area,mgnl:component,mgnl:user,mgnl:group,mgnl:role' |
  ./nodes.py -d 'http://gato-staging-testingsite.its.txstate.edu' -s 'testing-site-destroyer' |
  tee $name.node.txt |
  ./thrawler --conf=configs/gato-staging-testingsite.its.txstate.edu.conf --threads=8 --proxy='http://localhost' --crawl=false +header='Via: Proxy-HistoryCache/1.8.5' 2>>./log/thrawler.log |
  tee $name.json |
  ./stuc.py > $name.link.txt

# If this is the second phase then diff the
# before and after files. If they differ then
# overwrite the before file with the after
# file; so we do not get any more warnings
# for the same changes in links.
if [ $name == 'after' ]; then #after
  # Find missed transmogrifiers
  t=$'\t'; grep "$t[^$t]*mjdf38i3tv0b56vz" $name.link.txt > $name.miss.txt
  ./parity before.miss.txt after.miss.txt >links.miss.diff
  if [ -s "links.miss.diff" ]; then
    echo '========== Missed Transmogrified Links =========='
    cat links.miss.diff
    cat links.miss.diff | mail -s "thrawler missed transmogrifiers $(hostname -f)" "$emails"
  fi
  ./parity before.link.txt after.link.txt >links.diff
  if [ -s "links.diff" ]; then
    echo '========== Differing Links =========='
    cat links.diff
    cat links.diff | mail -s "thrawler link diffs $(hostname -f)" "$emails"
  fi
fi
