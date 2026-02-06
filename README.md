# Gorrent

Small Golang BitTorrent client.

> Gorrent: **Go** To**rrent**: too many other 'gotorrent' projects out there :D

Idea #1 from <https://codecrafters.io/blog/programming-project-ideas>, and built against the BitTorrent specification: <https://www.bittorrent.org/beps/bep_0003.html>

## BitTorrent process

The process for downloading a file via torrenting is:

- parse the [bencoded](https://en.wikipedia.org/wiki/Bencode) torrent file
  - this provides the announce urls, which are the 'trackers'
  - also an info dictionary, with details about the file(s) to be downloaded
- make a GET request to a tracker, passing values that identify you as a peer and giving the file info you want
- the tracker returns a peer list, ip address/port combos
- connect to peers, sending a handshake and exchanging bitfields - a bit structure indicating what pieces you and the peer have
- then exchange messages with peers to download and upload pieces of the file(s):
  - a request is sent, asking for an offset of a given length of a piece (typically 16kb at a time)
  - pieces are received, and these need to be saved to the local file

> All downloaders are also uploaders, as indicated by the last step above and the fact that you register with the tracker as a peer.
> The initial provider of a given torrent file or files starts a downloader to upload the data without needing to download anything (they already have the full file(s))

## Gorrent features

```
Usage: gorrent [options] <torrent-file>
  -v    enable verbose output
exit status 1
```

At present it only supports torrent files, and only single file downloads.
