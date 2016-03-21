#!/bin/bash
. emails

name="${1:-before}"
if [ "$name" != 'before' ] && [ "$name" != 'after' ]; then
	echo "USAGE: $0 [before|after]"
	exit 1
fi

echo 'http://gato-staging-testingsite.its.txstate.edu' |
  ./thrawler --conf=configs/staging-testingsite --threads=8 --proxy='http://localhost' +header='Via: Proxy-HistoryCache/1.8.5' |
  ./stuc.py |
  sort > $name.txt

# If this is the second phase then diff the
# before and after files. If they differ then
# overwrite the before file with the after
# file; so we do not get any more warnings
# for the same changes in links.
if [ $name == 'after' ]; then
	if ! (diff -U 0 before.txt after.txt >link.diff); then
		mv after.txt before.txt
		cat link.diff
		cat link.diff | mail -s "$(hostname -f) link diffs" "$emails"
	fi
fi
