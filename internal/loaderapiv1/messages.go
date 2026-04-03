package loaderapiv1

const (
	ContentType   = "application/x-protobuf"
	ProtocolValue = "loader.api.v1"
)

type ClientMeta struct {
	ClientName       string `protobuf:"bytes,1,opt,name=client_name,json=clientName,proto3" json:"client_name,omitempty"`
	ClientVersion    string `protobuf:"bytes,2,opt,name=client_version,json=clientVersion,proto3" json:"client_version,omitempty"`
	BuildFingerprint string `protobuf:"bytes,3,opt,name=build_fingerprint,json=buildFingerprint,proto3" json:"build_fingerprint,omitempty"`
	Platform         string `protobuf:"bytes,4,opt,name=platform,proto3" json:"platform,omitempty"`
	Locale           string `protobuf:"bytes,5,opt,name=locale,proto3" json:"locale,omitempty"`
	TransportMode    string `protobuf:"bytes,6,opt,name=transport_mode,json=transportMode,proto3" json:"transport_mode,omitempty"`
}

func (m *ClientMeta) Reset()         { *m = ClientMeta{} }
func (m *ClientMeta) String() string { return "ClientMeta" }
func (*ClientMeta) ProtoMessage()    {}

type ManifestRequest struct {
	ClientMeta *ClientMeta `protobuf:"bytes,1,opt,name=client_meta,json=clientMeta,proto3" json:"client_meta,omitempty"`
}

func (m *ManifestRequest) Reset()         { *m = ManifestRequest{} }
func (m *ManifestRequest) String() string { return "ManifestRequest" }
func (*ManifestRequest) ProtoMessage()    {}

type ManifestResponse struct {
	KeyId           string `protobuf:"bytes,1,opt,name=key_id,json=keyId,proto3" json:"key_id,omitempty"`
	TlsCertSha256   string `protobuf:"bytes,2,opt,name=tls_cert_sha256,json=tlsCertSha256,proto3" json:"tls_cert_sha256,omitempty"`
	ProtocolVersion string `protobuf:"bytes,3,opt,name=protocol_version,json=protocolVersion,proto3" json:"protocol_version,omitempty"`
	ServerName      string `protobuf:"bytes,4,opt,name=server_name,json=serverName,proto3" json:"server_name,omitempty"`
	GeneratedAt     string `protobuf:"bytes,5,opt,name=generated_at,json=generatedAt,proto3" json:"generated_at,omitempty"`
	Signature       []byte `protobuf:"bytes,6,opt,name=signature,proto3" json:"signature,omitempty"`
}

func (m *ManifestResponse) Reset()         { *m = ManifestResponse{} }
func (m *ManifestResponse) String() string { return "ManifestResponse" }
func (*ManifestResponse) ProtoMessage()    {}

type HandshakeRequest struct {
	ClientMeta          *ClientMeta `protobuf:"bytes,1,opt,name=client_meta,json=clientMeta,proto3" json:"client_meta,omitempty"`
	ClientPublicKey     []byte      `protobuf:"bytes,2,opt,name=client_public_key,json=clientPublicKey,proto3" json:"client_public_key,omitempty"`
	ClientNonce         []byte      `protobuf:"bytes,3,opt,name=client_nonce,json=clientNonce,proto3" json:"client_nonce,omitempty"`
	UnixMs              int64       `protobuf:"varint,4,opt,name=unix_ms,json=unixMs,proto3" json:"unix_ms,omitempty"`
	ClientIdentityKey   []byte      `protobuf:"bytes,5,opt,name=client_identity_key,json=clientIdentityKey,proto3" json:"client_identity_key,omitempty"`
	ClientIdentitySig   []byte      `protobuf:"bytes,6,opt,name=client_identity_sig,json=clientIdentitySig,proto3" json:"client_identity_sig,omitempty"`
	ClientIdentityKeyId string      `protobuf:"bytes,7,opt,name=client_identity_key_id,json=clientIdentityKeyId,proto3" json:"client_identity_key_id,omitempty"`
}

func (m *HandshakeRequest) Reset()         { *m = HandshakeRequest{} }
func (m *HandshakeRequest) String() string { return "HandshakeRequest" }
func (*HandshakeRequest) ProtoMessage()    {}

type HandshakeResponse struct {
	SessionID            string `protobuf:"bytes,1,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
	ServerPublicKey      []byte `protobuf:"bytes,2,opt,name=server_public_key,json=serverPublicKey,proto3" json:"server_public_key,omitempty"`
	ServerNonce          []byte `protobuf:"bytes,3,opt,name=server_nonce,json=serverNonce,proto3" json:"server_nonce,omitempty"`
	ProtocolVersion      string `protobuf:"bytes,4,opt,name=protocol_version,json=protocolVersion,proto3" json:"protocol_version,omitempty"`
	ServerVersion        string `protobuf:"bytes,5,opt,name=server_version,json=serverVersion,proto3" json:"server_version,omitempty"`
	Message              string `protobuf:"bytes,6,opt,name=message,proto3" json:"message,omitempty"`
	SessionExpiresUnixMs int64  `protobuf:"varint,7,opt,name=session_expires_unix_ms,json=sessionExpiresUnixMs,proto3" json:"session_expires_unix_ms,omitempty"`
}

func (m *HandshakeResponse) Reset()         { *m = HandshakeResponse{} }
func (m *HandshakeResponse) String() string { return "HandshakeResponse" }
func (*HandshakeResponse) ProtoMessage()    {}

type SealedRequest struct {
	SessionID  string `protobuf:"bytes,1,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
	Nonce      []byte `protobuf:"bytes,2,opt,name=nonce,proto3" json:"nonce,omitempty"`
	Ciphertext []byte `protobuf:"bytes,3,opt,name=ciphertext,proto3" json:"ciphertext,omitempty"`
}

func (m *SealedRequest) Reset()         { *m = SealedRequest{} }
func (m *SealedRequest) String() string { return "SealedRequest" }
func (*SealedRequest) ProtoMessage()    {}

type SealedResponse struct {
	Nonce      []byte `protobuf:"bytes,1,opt,name=nonce,proto3" json:"nonce,omitempty"`
	Ciphertext []byte `protobuf:"bytes,2,opt,name=ciphertext,proto3" json:"ciphertext,omitempty"`
}

func (m *SealedResponse) Reset()         { *m = SealedResponse{} }
func (m *SealedResponse) String() string { return "SealedResponse" }
func (*SealedResponse) ProtoMessage()    {}

type HealthzRequest struct {
	ClientMeta *ClientMeta `protobuf:"bytes,1,opt,name=client_meta,json=clientMeta,proto3" json:"client_meta,omitempty"`
}

func (m *HealthzRequest) Reset()         { *m = HealthzRequest{} }
func (m *HealthzRequest) String() string { return "HealthzRequest" }
func (*HealthzRequest) ProtoMessage()    {}

type HealthzResponse struct {
	Ok              bool   `protobuf:"varint,1,opt,name=ok,proto3" json:"ok,omitempty"`
	ServerName      string `protobuf:"bytes,2,opt,name=server_name,json=serverName,proto3" json:"server_name,omitempty"`
	ServerVersion   string `protobuf:"bytes,3,opt,name=server_version,json=serverVersion,proto3" json:"server_version,omitempty"`
	ProtocolVersion string `protobuf:"bytes,4,opt,name=protocol_version,json=protocolVersion,proto3" json:"protocol_version,omitempty"`
	Message         string `protobuf:"bytes,5,opt,name=message,proto3" json:"message,omitempty"`
}

func (m *HealthzResponse) Reset()         { *m = HealthzResponse{} }
func (m *HealthzResponse) String() string { return "HealthzResponse" }
func (*HealthzResponse) ProtoMessage()    {}

type RuntimeProfileRequest struct {
	ClientMeta *ClientMeta `protobuf:"bytes,1,opt,name=client_meta,json=clientMeta,proto3" json:"client_meta,omitempty"`
}

func (m *RuntimeProfileRequest) Reset()         { *m = RuntimeProfileRequest{} }
func (m *RuntimeProfileRequest) String() string { return "RuntimeProfileRequest" }
func (*RuntimeProfileRequest) ProtoMessage()    {}

type RuntimeProfileResponse struct {
	Ok             bool   `protobuf:"varint,1,opt,name=ok,proto3" json:"ok,omitempty"`
	Message        string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	ChannelId      int64  `protobuf:"varint,3,opt,name=channel_id,json=channelId,proto3" json:"channel_id,omitempty"`
	AppId          int64  `protobuf:"varint,4,opt,name=app_id,json=appId,proto3" json:"app_id,omitempty"`
	CpId           string `protobuf:"bytes,5,opt,name=cp_id,json=cpId,proto3" json:"cp_id,omitempty"`
	CpAppId        string `protobuf:"bytes,6,opt,name=cp_app_id,json=cpAppId,proto3" json:"cp_app_id,omitempty"`
	CpAppKey       string `protobuf:"bytes,7,opt,name=cp_app_key,json=cpAppKey,proto3" json:"cp_app_key,omitempty"`
	ServerId       int64  `protobuf:"varint,8,opt,name=server_id,json=serverId,proto3" json:"server_id,omitempty"`
	ChannelVersion string `protobuf:"bytes,9,opt,name=channel_version,json=channelVersion,proto3" json:"channel_version,omitempty"`
	GameVer        string `protobuf:"bytes,10,opt,name=game_ver,json=gameVer,proto3" json:"game_ver,omitempty"`
	VersionCode    int64  `protobuf:"varint,11,opt,name=version_code,json=versionCode,proto3" json:"version_code,omitempty"`
	SdkVer         string `protobuf:"bytes,12,opt,name=sdk_ver,json=sdkVer,proto3" json:"sdk_ver,omitempty"`
}

func (m *RuntimeProfileResponse) Reset()         { *m = RuntimeProfileResponse{} }
func (m *RuntimeProfileResponse) String() string { return "RuntimeProfileResponse" }
func (*RuntimeProfileResponse) ProtoMessage()    {}

type ScanExecuteRequest struct {
	ClientMeta   *ClientMeta `protobuf:"bytes,1,opt,name=client_meta,json=clientMeta,proto3" json:"client_meta,omitempty"`
	Ticket       string      `protobuf:"bytes,2,opt,name=ticket,proto3" json:"ticket,omitempty"`
	AccessKey    string      `protobuf:"bytes,3,opt,name=access_key,json=accessKey,proto3" json:"access_key,omitempty"`
	AsteriskName string      `protobuf:"bytes,4,opt,name=asterisk_name,json=asteriskName,proto3" json:"asterisk_name,omitempty"`
	UID          int64       `protobuf:"varint,5,opt,name=uid,proto3" json:"uid,omitempty"`
}

func (m *ScanExecuteRequest) Reset()         { *m = ScanExecuteRequest{} }
func (m *ScanExecuteRequest) String() string { return "ScanExecuteRequest" }
func (*ScanExecuteRequest) ProtoMessage()    {}

type ScanExecuteResponse struct {
	RequestID   string `protobuf:"bytes,1,opt,name=request_id,json=requestId,proto3" json:"request_id,omitempty"`
	Retcode     int64  `protobuf:"varint,2,opt,name=retcode,proto3" json:"retcode,omitempty"`
	Message     string `protobuf:"bytes,3,opt,name=message,proto3" json:"message,omitempty"`
	Confirmed   bool   `protobuf:"varint,4,opt,name=confirmed,proto3" json:"confirmed,omitempty"`
	NeedRelogin bool   `protobuf:"varint,5,opt,name=need_relogin,json=needRelogin,proto3" json:"need_relogin,omitempty"`
	Retryable   bool   `protobuf:"varint,6,opt,name=retryable,proto3" json:"retryable,omitempty"`
}

func (m *ScanExecuteResponse) Reset()         { *m = ScanExecuteResponse{} }
func (m *ScanExecuteResponse) String() string { return "ScanExecuteResponse" }
func (*ScanExecuteResponse) ProtoMessage()    {}
