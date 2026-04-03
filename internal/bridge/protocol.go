package bridge

type ClientMeta struct {
	ClientName       string `json:"clientName,omitempty"`
	ClientVersion    string `json:"clientVersion,omitempty"`
	BuildFingerprint string `json:"buildFingerprint,omitempty"`
	Platform         string `json:"platform,omitempty"`
	Locale           string `json:"locale,omitempty"`
	TransportMode    string `json:"transportMode,omitempty"`
}

type ScanRequest struct {
	Ticket       string     `json:"ticket,omitempty"`
	UID          int64      `json:"uid,omitempty"`
	AccessKey    string     `json:"accessKey,omitempty"`
	AsteriskName string     `json:"asteriskName,omitempty"`
	LoaderAPIURL string     `json:"loaderApiUrl,omitempty"`
	ClientMeta   ClientMeta `json:"clientMeta"`
}

type ScanResponse struct {
	Retcode int64  `json:"retcode,omitempty"`
	Message string `json:"message,omitempty"`
}

type RuntimeProfile struct {
	ChannelID      int64  `json:"channelId,omitempty"`
	AppID          int64  `json:"appId,omitempty"`
	CPID           string `json:"cpId,omitempty"`
	CPAppID        string `json:"cpAppId,omitempty"`
	CPAppKey       string `json:"cpAppKey,omitempty"`
	ServerID       int64  `json:"serverId,omitempty"`
	ChannelVersion string `json:"channelVersion,omitempty"`
	GameVer        string `json:"gameVer,omitempty"`
	VersionCode    int64  `json:"versionCode,omitempty"`
	SDKVer         string `json:"sdkVer,omitempty"`
}
