package tracker

type PeerInfo struct {
	Id   string
	IP   string
	Port uint16
}

type TrackerResponse struct {
	LocalID   []byte
	LocalPort uint16
	Peers     []PeerInfo
	Interval  int
}

var nil_resp TrackerResponse
var nil_info []PeerInfo
