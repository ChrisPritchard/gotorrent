package torrent_files

type TorrentMetadata struct {
	Announcers  []string
	InfoHash    [20]byte
	Name        string
	PieceLength int
	Pieces      []string
	Length      int
	Files       []TorrentFile
}

type TorrentFile struct {
	Path   []string
	Length int
}
