# upgraderr

http://upgraderr.upgraderr:6940/api/upgrade
```
{ "host":"http://qbittorrent.cat:8080",
  "user":"zees",
  "password":"bsmom",
  "name":"{{ .TorrentName | js }}" }
```

* Possible returns
  * 200 unique
  * 250 cross
* Informational returns
  * 201-208

http://upgraderr.upgraderr:6940/api/cross
```
{  "host":"http://qbittorrent.cat:8080",
   "user":"zees",
   "password":"bsmom",
   "name":"{{ .TorrentName | js }}",
   "hash":"{{ .TorrentHash }}",
   "torrent":"{{ .TorrentDataRawBytes | js }}" }
```

* Possible returns
  * 200 ok
* Error returns
  * 400-499

### Experimental endpoints below
http://upgraderr.upgraderr:6940/api/clean
```
{ "host":"http://qbittorrent.cat:8080",
  "user":"zees",
  "password":"bsmom" }
```

* Possible returns
  * 200 ok
  * 205 nothing to remove
* Error returns
  * 400-499

http://upgraderr.upgraderr:6940/api/unregistered
```
{ "host":"http://qbittorrent.cat:8080",
  "user":"zees",
  "password":"bsmom" }
```

* Possible returns
  * 200 ok
* Error returns
  * 400-499

http://upgraderr.upgraderr:6940/api/autobrr/filterupdate
```
{ "host":"http://qbittorrent.cat:8080",
  "user":"zees",
  "password":"bsmom",
  "autobrrhost":"http://autobrr.autobrr:7474", 
  "apikey":"YnNtb21pc3RoZWJlc3Q=",
  "filterid":69 }
```

* Possible returns
  * 200 ok
* Error returns
  * 400-499

http://upgraderr.upgraderr:6940/api/expression
```
{ "host":"http://qbittorrent.cat:8080",
  "user":"zees",
  "password":"bsmom",
  "action":"start",
  "query":"LastActivity != 0 && State(State) == 'stalledUP' && Now() - LastActivity > 800 && ((SeedingTime > 7776 && (NumComplete > 12 || NumIncomplete > 9)) || (SeedingTime > 10368 && (NumComplete > 8 || NumIncomplete > 6)))"
 }
```

```
{ "host":"http://qbittorrent.cat:8080",
  "user":"zees",
  "password":"bsmom",
  "action":"reannounce",
  "query":"DisableCrossseed() && State(State) in ['stalledDL', 'forcedDL', 'downloading'] && NumLeechs + NumSeeds < 3"
 }
```

```
{ "host":"http://qbittorrent.cat:8080",
  "user":"zees",
  "password":"bsmom",
  "action":"tagadd",
  "subject":"dageraad",
  "query":"DisableCrossseed() && State(State) == 'downloading' && Tags not contains 'forkedRiver' && NumLeechs + NumSeeds > 7"
 }
```

* Possible returns
  * 200 ok
* Error returns
  * 400-499
* Language documentation
  * https://expr.medv.io/docs/Language-Definition
* Specifiers available
  * https://github.com/autobrr/go-qbittorrent/blob/f9978be1e0e1e8db4b576b27ecae110b1b37d5fc/domain.go#L7
* Actions available
  * delete, deletedata, forcestart, normalstart, start, pause, reannounce, recheck, test (default)
* Actions with Subjects
  * category, tagadd, tagdel
* Custom script functions
  * Now()
      - Unix timestamp
  * State(State)
      - Converts the torrent state to a string
  * DisableCrossseed()
      - Naive matching
  * ResultLimit(int)
      - Limits results to process after the classification stage.
  * SpaceAvailable('/my/path'), SpaceFree('/my/path'), SpaceTotal('/my/path'), SpaceUsed('/my/path')
      - Returns bytes from each respective function
  * TitleParse(string)
      - Parses a title, to return fields found in [moistari/rls](https://github.com/moistari/rls/blob/v0.5.9/rls.go#L22)
  * TitleParsed()
      - Parses the present title, to return fields found in [moistari/rls](https://github.com/moistari/rls/blob/v0.5.9/rls.go#L22)
 
<!-- end of the list -->
