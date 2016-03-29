#!/bin/bash
. emails

name="${1:-before}"
if [ "$name" != 'before' ] && [ "$name" != 'after' ]; then
  echo "USAGE: $0 [before|after]"
  exit 1
fi

# Teeing off json for thrawler debugging.
# Need to remove cache busting hashes as
# currently they differ between builds.
echo 'http://gato-staging-testingsite.its.txstate.edu' |
  ./thrawler --conf=configs/gato-staging-testingsite.its.txstate.edu.conf --threads=8 --proxy='http://localhost' +header='Via: Proxy-HistoryCache/1.8.5' |
  tee $name.json |
  ./stuc.py |
  sed 's/magnoliaAssets\/cache[0-9a-z]\+\//magnoliaAssets\/cache...\//g; s/cache[0-9a-z]\+\/imagehandler\//cache...\/imagehandler\//g' |
  sort > $name.txt

# If this is the second phase then diff the
# before and after files. If they differ then
# overwrite the before file with the after
# file; so we do not get any more warnings
# for the same changes in links.
if [ $name == 'after' ]; then
  # Find missed transmogrifiers
  t=$'\t'; grep "$t[^$t]*mjdf38i3tv0b56vz" $name.txt > $name.miss.txt
	if ! (diff -U 0 before.miss.txt after.miss.txt >links.miss.diff); then
    echo '========== Missed Transmogrified Links =========='
    grep -v '^@' links.miss.diff
    mv after.miss.txt before.miss.txt
    grep -v '^@' links.miss.diff | mail -s "$(hostname -f) missed transmogrifiers" "$emails"
  fi 
  if ! (diff -U 0 before.txt after.txt >links.diff); then
    echo '========== Differing Links =========='
    grep -v '^@' links.diff
    mv after.txt before.txt
    grep -v '^@' links.diff | mail -s "$(hostname -f) link diffs" "$emails"
  fi
else
  >$name.miss.txt
fi
