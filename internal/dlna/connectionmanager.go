package dlna

import (
	"fmt"
	"net/http"
	"strings"
)

// ConnectionManagerService implements UPnP ConnectionManager:1.
type ConnectionManagerService struct{}

// NewConnectionManagerService creates a new ConnectionManager.
func NewConnectionManagerService() *ConnectionManagerService {
	return &ConnectionManagerService{}
}

// HandleControl processes SOAP action requests for ConnectionManager.
func (cm *ConnectionManagerService) HandleControl(w http.ResponseWriter, r *http.Request) {
	soapAction := r.Header.Get("SOAPAction")
	soapAction = strings.Trim(soapAction, "\"")

	switch {
	case strings.HasSuffix(soapAction, "#GetProtocolInfo"):
		cm.handleGetProtocolInfo(w)
	case strings.HasSuffix(soapAction, "#GetCurrentConnectionIDs"):
		cm.handleGetCurrentConnectionIDs(w)
	case strings.HasSuffix(soapAction, "#GetCurrentConnectionInfo"):
		cm.handleGetCurrentConnectionInfo(w)
	default:
		w.WriteHeader(http.StatusNotImplemented)
	}
}

func (cm *ConnectionManagerService) handleGetProtocolInfo(w http.ResponseWriter) {
	// Report supported protocols: common video, audio, and image formats
	sourceProtocols := strings.Join([]string{
		"http-get:*:video/mp4:*",
		"http-get:*:video/x-matroska:*",
		"http-get:*:video/mpeg:*",
		"http-get:*:video/avi:*",
		"http-get:*:video/webm:*",
		"http-get:*:video/mp2t:*",
		"http-get:*:audio/mpeg:*",
		"http-get:*:audio/mp4:*",
		"http-get:*:audio/flac:*",
		"http-get:*:audio/ogg:*",
		"http-get:*:image/jpeg:*",
		"http-get:*:image/png:*",
	}, ",")

	cm.soapResponse(w, "GetProtocolInfo", map[string]string{
		"Source":         sourceProtocols,
		"Sink":           "",
	})
}

func (cm *ConnectionManagerService) handleGetCurrentConnectionIDs(w http.ResponseWriter) {
	cm.soapResponse(w, "GetCurrentConnectionIDs", map[string]string{
		"ConnectionIDs": "0",
	})
}

func (cm *ConnectionManagerService) handleGetCurrentConnectionInfo(w http.ResponseWriter) {
	cm.soapResponse(w, "GetCurrentConnectionInfo", map[string]string{
		"RcsID":                 "-1",
		"AVTransportID":        "-1",
		"ProtocolInfo":         "",
		"PeerConnectionManager": "",
		"PeerConnectionID":     "-1",
		"Direction":            "Output",
		"Status":               "OK",
	})
}

// HandleSCPD serves the ConnectionManager service description XML.
func (cm *ConnectionManagerService) HandleSCPD(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Write([]byte(connectionManagerSCPD))
}

func (cm *ConnectionManagerService) soapResponse(w http.ResponseWriter, action string, params map[string]string) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">`)
	sb.WriteString(`<s:Body>`)
	sb.WriteString(fmt.Sprintf(`<u:%sResponse xmlns:u="urn:schemas-upnp-org:service:ConnectionManager:1">`, action))
	for k, v := range params {
		sb.WriteString(fmt.Sprintf(`<%s>%s</%s>`, k, xmlEscape(v), k))
	}
	sb.WriteString(fmt.Sprintf(`</u:%sResponse>`, action))
	sb.WriteString(`</s:Body></s:Envelope>`)
	w.Write([]byte(sb.String()))
}

const connectionManagerSCPD = `<?xml version="1.0" encoding="UTF-8"?>
<scpd xmlns="urn:schemas-upnp-org:service-1-0">
<specVersion><major>1</major><minor>0</minor></specVersion>
<actionList>
<action><name>GetProtocolInfo</name>
<argumentList>
<argument><name>Source</name><direction>out</direction><relatedStateVariable>SourceProtocolInfo</relatedStateVariable></argument>
<argument><name>Sink</name><direction>out</direction><relatedStateVariable>SinkProtocolInfo</relatedStateVariable></argument>
</argumentList>
</action>
<action><name>GetCurrentConnectionIDs</name>
<argumentList>
<argument><name>ConnectionIDs</name><direction>out</direction><relatedStateVariable>CurrentConnectionIDs</relatedStateVariable></argument>
</argumentList>
</action>
<action><name>GetCurrentConnectionInfo</name>
<argumentList>
<argument><name>ConnectionID</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_ConnectionID</relatedStateVariable></argument>
<argument><name>RcsID</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_RcsID</relatedStateVariable></argument>
<argument><name>AVTransportID</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_AVTransportID</relatedStateVariable></argument>
<argument><name>ProtocolInfo</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_ProtocolInfo</relatedStateVariable></argument>
<argument><name>PeerConnectionManager</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_ConnectionManager</relatedStateVariable></argument>
<argument><name>PeerConnectionID</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_ConnectionID</relatedStateVariable></argument>
<argument><name>Direction</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_Direction</relatedStateVariable></argument>
<argument><name>Status</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_ConnectionStatus</relatedStateVariable></argument>
</argumentList>
</action>
</actionList>
<serviceStateTable>
<stateVariable sendEvents="yes"><name>SourceProtocolInfo</name><dataType>string</dataType></stateVariable>
<stateVariable sendEvents="yes"><name>SinkProtocolInfo</name><dataType>string</dataType></stateVariable>
<stateVariable sendEvents="yes"><name>CurrentConnectionIDs</name><dataType>string</dataType></stateVariable>
<stateVariable sendEvents="no"><name>A_ARG_TYPE_ConnectionStatus</name><dataType>string</dataType></stateVariable>
<stateVariable sendEvents="no"><name>A_ARG_TYPE_ConnectionManager</name><dataType>string</dataType></stateVariable>
<stateVariable sendEvents="no"><name>A_ARG_TYPE_Direction</name><dataType>string</dataType></stateVariable>
<stateVariable sendEvents="no"><name>A_ARG_TYPE_ProtocolInfo</name><dataType>string</dataType></stateVariable>
<stateVariable sendEvents="no"><name>A_ARG_TYPE_ConnectionID</name><dataType>i4</dataType></stateVariable>
<stateVariable sendEvents="no"><name>A_ARG_TYPE_AVTransportID</name><dataType>i4</dataType></stateVariable>
<stateVariable sendEvents="no"><name>A_ARG_TYPE_RcsID</name><dataType>i4</dataType></stateVariable>
</serviceStateTable>
</scpd>`
