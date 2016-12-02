#!/usr/bin/python3
import sys, json, re


# keep track of embedded links so that we may print them only once.
embedded = {}

for line in sys.stdin:
  data = json.loads(line)
  if data["lvl"] == 3:
    printable = True
    src = data["src"]
    tag = data["tag"]
    url = data["url"]
    code = str(data["code"])
    # Consolidate entries for embedded links.
    # NOTE: Div ids refer to a group of embedded links; so use that to consolidate them.
    #   Calendar events within: /div#aAbBcCdDeE.column_paragraph/div.gato-events/      => tag=gato-events(aAbBcCdDeE)
    #   Twitter feeds within: /div#bBcCdDeEfF.column_paragraph/div.gato-twitter-feed/  => tag=gato-twitter-feed(bBcCdDeEfF)
    #   RSS feed: /div#cCdDeEfFgG.column_paragraph/div.gato-rss-item)/                 => tag=gato-rss-item(cCdDeEfFgG)
    m = re.match('.*?/div#([a-zA-Z0-9]{8,12})\.column_paragraph/div\.(gato-events|gato-twitter-feed|gato-rss-item)/.*', tag)
    if m:
      url = ""
      tag = m.group(2)+"("+m.group(1)+")"
      key = src+"/"+tag
      if m.group(2) == "gato-events":
        if key in embedded:
          printable = False
        else:
          embedded[key] = True
      elif m.group(2) == "gato-twitter-feed":
        if key in embedded:
          printable = False
        else:
          embedded[key] = True
      elif m.group(2) == "gato-rss-item":
        if key in embedded:
          printable = False
        else:
          embedded[key] = True
    else:
      # Filter out cache busting hashes, as they refer to the same link
      # with a different hash string refering to the build.
      # NOTE: Thrawler does NOT already modify all the imagehandler paths
      # if they end up redirecting other hosts such as etcalender. This
      # is possible as such links are being redirected to the imagehandler
      # service via the cache boxes.
      url = re.sub('/magnoliaAssets/cache[0-9a-z]+/', '/magnoliaAssets/cache.../', url)
      url = re.sub('/cache[0-9a-z]+/imagehandler/', '/cache.../imagehandler/', url)
      # For non embedded links filter tag fields to only include the
      # ending path, as we do NOT care where on the page we found them,
      # but rather what element and it's attribute we found them in.
      tags = tag.split("/")
      tag = tags[-1]
    if printable:
      print(src + "\t" + tag + "\t" + url + "\t" + code)
