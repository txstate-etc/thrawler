# thrawler
A threaded crawler to help with checking Gato CMS link generation before and after updates

**Example of how to run thrawler**
The following command will tell thrawler to start scanning the gato-staging-testingsite.its.txstate.edu site. Eight goroutines will be utilized, i.e. no more than 8 requests will be made at one time to the site. Requests will all be sent to the gato-public-st.tr.txstate.edu loadbalancer. More than one header may be added, but in this case only the Via header is added to tell Gato to treat the request as if it is coming from the cache boxes. The logs will be piped through the stuc.py script to convert the output into a tab delmited one with only source, tag, url, and status code fields. The the output is sorted and saved to the links text. This allows us to save all the links found and compare before and after updates.

```
echo 'http://gato-staging-testingsite.its.txstate.edu' |
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
