package tracker

type PeerInfo struct {
	Id   string
	IP   string
	Port uint16
}

type TrackerResponse struct {
	Peers    []PeerInfo
	Interval int
}

var nil_resp TrackerResponse
var nil_info []PeerInfo
