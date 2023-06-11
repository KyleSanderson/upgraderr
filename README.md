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
