# Gorrent

Small Golang BitTorrent client.

Idea #1 from <https://codecrafters.io/blog/programming-project-ideas>, and built against the BitTorrent specification: <https://www.bittorrent.org/beps/bep_0003.html>

## BitTorrent process

The process for downloading a file via torrenting is:

- parse the [bencoded](https://en.wikipedia.org/wiki/Bencode) torrent file
  - this provides the announce urls, which are the 'trackers'
  - also an info dictionary, with details about the file(s) to be downloaded
- make a GET request to a tracker, passing values that identify you as a peer and giving the file info you want
- the tracker returns a peer list, ip address/port combos
- connect to peers, sending a handshake and verifying their status
- then exchange messages with peers to download and upload pieces of the file(s)

all downloaders are also uploaders, as indicated by the last step above and the fact that you register with the tracker as a peer

the initial provider of a given torrent file or files starts a downloader to upload the data without needing to download anything (they already have the full file(s))
