#!/bin/bash
. .env

name="${1:-before}"
if [ "$name" != 'before' ] && [ "$name" != 'after' ]; then
  echo "USAGE: $0 [before|after]"
  exit 1
fi

# Tee off json for thrawler debugging.
# Tee off node info for magnolia RESTful
# debugging.
# Need to remove cache busting hashes as
# currently they differ between builds.
curl --user "$magusr" -H 'Accept: application/json' 'http://localhost:8080/mjdf38i3tv0b56vz/.rest/nodes/v1/website/testing-site-destroyer?depth=999&excludeNodeTypes=mgnl:resource,mgnl:metaData,mgnl:content,mgnl:contentNode,mgnl:area,mgnl:component,mgnl:user,mgnl:group,mgnl:role' |
  ./nodes.py -d 'http://gato-staging-testingsite.its.txstate.edu' -s 'testing-site-destroyer' |
  tee $name.node.txt |
  ./thrawler --conf=configs/gato-staging-testingsite.its.txstate.edu.conf --threads=8 --proxy='http://localhost' --crawl=false +header='Via: Proxy-HistoryCache/1.8.5' |
  tee $name.json |
  ./stuc.py |
  sed 's/magnoliaAssets\/cache[0-9a-z]\+\//magnoliaAssets\/cache...\//g; s/cache[0-9a-z]\+\/imagehandler\//cache...\/imagehandler\//g' |
  sort > $name.link.txt

# If this is the second phase then diff the
# before and after files. If they differ then
# overwrite the before file with the after
# file; so we do not get any more warnings
# for the same changes in links.
if [ $name == 'after' ]; then
  # Find missed transmogrifiers
  t=$'\t'; grep "$t[^$t]*mjdf38i3tv0b56vz" $name.link.txt > $name.miss.txt
  ./parity -b before.miss.txt -a after.miss.txt >links.miss.diff
  if [ -s "links.miss.diff" ]; then
    echo '========== Missed Transmogrified Links =========='
    cat links.miss.diff
    mv after.miss.txt before.miss.txt
    cat links.miss.diff | mail -s "thrawler missed transmogrifiers $(hostname -f)" "$emails"
  fi
  ./parity -b before.link.txt -a after.link.txt > links.diff
  if [ -s "links.diff" ]; then
    echo '========== Differing Links =========='
    cat links.diff
    mv after.link.txt before.link.txt
    cat links.diff | mail -s "thrawler link diffs $(hostname -f)" "$emails"
  fi
else
  >$name.miss.txt
fi
