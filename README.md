# thrawler
A threaded crawler to help with checking Gato CMS link generation before and after updates

**Disclaimer:**
Please do not point this application to other peoples production sites and take care if you use it on your own sites. This crawler has been designed to access parts of our staging sites as quickly as possible, and is thus not throttled. Nor does this crawler take into consideration robot.txt guidelines, as it needs to verify all of our staging pages regardless of these restrictions. This is not good behavior for a general use crawler, and we cannot be held responsible if used inappropriately.

**Example of how to run thrawler**
The following command will tell thrawler to start scanning the gato-staging-testingsite and gato-staging-mainsite2012 sites on gato. Eight goroutines will be utilized, i.e. no more than 8 requests will be made at one time to the site. Requests will all be sent to the gato-public-st.tr.txstate.edu loadbalancer. More than one header may be added, but in this case only the Via header is utilized to tell Gato to treat the request as if it is coming from the cache boxes. The logs will be piped through the stuc.py script to convert the stream into tab delmited output with only source, tag, url, and status code fields. This output is then sorted and saved to the links.txt file. This lets us save all the links found in a way that allows us to compare before and after Gato updates no matter the order in which the pages where originally scanned.

```
echo -e 'http://gato-staging-testingsite.its.txstate.edu\nhttp://gato-staging-mainsite2012.its.txstate.edu' |
  ./thrawler --conf=configs/staging-testingsite --threads=8 --proxy='http://gato-public-st.tr.txstate.edu' +header='Via: Proxy-HistoryCache/1.8.5' |
  ./stuc.py |
  sort > links.txt
```

**Example of configs/staging-testingsite file:**
```
# Process gato-staging-docs, gato-staging-testinsite and gato-staging-mainsite2012 sites)
^http://(gato-staging-docs|gato-staging-testingsite|gato-staging-mainsite2012)\.its\.txstate\.edu($|/)
^(https?:)//[^/]+/cache[a-z0-9]+/imagehandler/scaler/([^?]+)	${1}//${2}
^(https?:)//testing-site-destroyer.its.txstate.edu($|/)	${1}//gato-staging-testingsite.its.txstate.edu${2}
^(https?:)//www.txstate.edu($|/)	${1}//gato-staging-mainsite2012.its.txstate.edu${2}
```

**Example of thrawler json logged output:**
```
{"app":"thrawler","code":200,"err":"","lvl":3,"msg":"req","net":"true","path":"/","src":"","t":"2016-03-17T20:32:18.398855487-05:00","tag":"","thd":0,"type":"GET","url":"http://gato-staging-testingsite.its.txstate.edu/"}
{"app":"thrawler","code":200,"err":"","lvl":3,"msg":"req","net":"true","path":"/.resources/gato-lib/js/modal.js","src":"http://gato-staging-testingsite.its.txstate.edu/","t":"2016-03-17T20:32:18.426501316-05:00","tag":"script","thd":4,"type":"HEAD","url":"http://gato-staging-mainsite2012.its.txstate.edu/.resources/gato-lib/js/modal.js"}
...
```

**stuc.py python script:**
The stuc.py python script converts thrawler log output to a tab delimited version with only source, tag, url and status code fields.

**Install python3 on RHEL6**
```
wget https://www.python.org/ftp/python/3.5.1/Python-3.5.1.tar.xz
tar xf Python-3.*
cd Python-3.*
./configure
make
# Install as python3.* so don't overwrite default python executable as yum needs python to be 2.x on RHEL6
make altinstall
mv /usr/local/bin/python3.5 /usr/bin/python3.5
ln -s python3.5 /usr/bin/python3
```
