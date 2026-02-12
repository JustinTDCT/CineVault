package dlna

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// DIDLItem represents a media item in DIDL-Lite XML format.
type DIDLItem struct {
	ID         string
	ParentID   string
	Title      string
	Class      string // e.g. "object.item.videoItem.movie"
	MimeType   string
	URL        string
	Duration   string // "HH:MM:SS"
	Resolution string
	Bitrate    int
	Size       int64
}

// DIDLContainer represents a DLNA container (folder/library).
type DIDLContainer struct {
	ID       string
	ParentID string
	Title    string
	Count    int
}

// MediaProvider is the interface the ContentDirectory uses to query media.
type MediaProvider interface {
	GetLibraries() ([]DIDLContainer, error)
	GetLibraryItems(libraryID string) ([]DIDLItem, error)
}

// ContentDirectoryService implements UPnP ContentDirectory:1.
type ContentDirectoryService struct {
	serverAddr string
	provider   MediaProvider
}

// NewContentDirectoryService creates a new ContentDirectory service.
func NewContentDirectoryService(serverAddr string, provider MediaProvider) *ContentDirectoryService {
	return &ContentDirectoryService{
		serverAddr: strings.TrimRight(serverAddr, "/"),
		provider:   provider,
	}
}

// HandleControl processes SOAP action requests for ContentDirectory.
func (cd *ContentDirectoryService) HandleControl(w http.ResponseWriter, r *http.Request) {
	soapAction := r.Header.Get("SOAPAction")
	soapAction = strings.Trim(soapAction, "\"")

	switch {
	case strings.HasSuffix(soapAction, "#Browse"):
		cd.handleBrowse(w, r)
	case strings.HasSuffix(soapAction, "#GetSystemUpdateID"):
		cd.handleGetSystemUpdateID(w, r)
	case strings.HasSuffix(soapAction, "#Search"):
		cd.handleSearch(w, r)
	default:
		w.WriteHeader(http.StatusNotImplemented)
	}
}

func (cd *ContentDirectoryService) handleBrowse(w http.ResponseWriter, r *http.Request) {
	// Parse SOAP envelope to get ObjectID
	objectID := "0" // Root
	if body, err := parseSOAPBody(r); err == nil {
		if oid := extractXMLTag(body, "ObjectID"); oid != "" {
			objectID = oid
		}
	}

	var result string
	var count, total int

	if objectID == "0" {
		// Root: return libraries as containers
		containers, err := cd.provider.GetLibraries()
		if err != nil {
			cd.soapError(w, 501, "Action Failed")
			return
		}
		result = cd.buildContainerDIDL(containers)
		count = len(containers)
		total = count
	} else {
		// Library items
		items, err := cd.provider.GetLibraryItems(objectID)
		if err != nil {
			cd.soapError(w, 501, "Action Failed")
			return
		}
		result = cd.buildItemDIDL(items)
		count = len(items)
		total = count
	}

	cd.soapResponse(w, "Browse", map[string]string{
		"Result":         result,
		"NumberReturned": fmt.Sprintf("%d", count),
		"TotalMatches":   fmt.Sprintf("%d", total),
		"UpdateID":       "1",
	})
}

func (cd *ContentDirectoryService) handleGetSystemUpdateID(w http.ResponseWriter, r *http.Request) {
	cd.soapResponse(w, "GetSystemUpdateID", map[string]string{
		"Id": "1",
	})
}

func (cd *ContentDirectoryService) handleSearch(w http.ResponseWriter, r *http.Request) {
	// Minimal search implementation — return empty
	cd.soapResponse(w, "Search", map[string]string{
		"Result":         `<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/"></DIDL-Lite>`,
		"NumberReturned": "0",
		"TotalMatches":   "0",
		"UpdateID":       "1",
	})
}

func (cd *ContentDirectoryService) buildContainerDIDL(containers []DIDLContainer) string {
	var sb strings.Builder
	sb.WriteString(`<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/">`)
	for _, c := range containers {
		sb.WriteString(fmt.Sprintf(`<container id="%s" parentID="%s" childCount="%d" restricted="true"><dc:title>%s</dc:title><upnp:class>object.container.storageFolder</upnp:class></container>`,
			xmlEscape(c.ID), xmlEscape(c.ParentID), c.Count, xmlEscape(c.Title)))
	}
	sb.WriteString(`</DIDL-Lite>`)
	return sb.String()
}

func (cd *ContentDirectoryService) buildItemDIDL(items []DIDLItem) string {
	var sb strings.Builder
	sb.WriteString(`<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns:dlna="urn:schemas-dlna-org:metadata-1-0/">`)
	for _, item := range items {
		class := item.Class
		if class == "" {
			class = "object.item.videoItem"
		}
		sb.WriteString(fmt.Sprintf(`<item id="%s" parentID="%s" restricted="true"><dc:title>%s</dc:title><upnp:class>%s</upnp:class>`,
			xmlEscape(item.ID), xmlEscape(item.ParentID), xmlEscape(item.Title), class))
		// Resource element
		sb.WriteString(fmt.Sprintf(`<res protocolInfo="http-get:*:%s:DLNA.ORG_OP=01;DLNA.ORG_CI=0"`, item.MimeType))
		if item.Duration != "" {
			sb.WriteString(fmt.Sprintf(` duration="%s"`, item.Duration))
		}
		if item.Resolution != "" {
			sb.WriteString(fmt.Sprintf(` resolution="%s"`, item.Resolution))
		}
		if item.Size > 0 {
			sb.WriteString(fmt.Sprintf(` size="%d"`, item.Size))
		}
		sb.WriteString(fmt.Sprintf(`>%s</res>`, xmlEscape(item.URL)))
		sb.WriteString(`</item>`)
	}
	sb.WriteString(`</DIDL-Lite>`)
	return sb.String()
}

// HandleSCPD serves the ContentDirectory service description XML.
func (cd *ContentDirectoryService) HandleSCPD(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Write([]byte(contentDirectorySCPD))
}

func (cd *ContentDirectoryService) soapResponse(w http.ResponseWriter, action string, params map[string]string) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">`)
	sb.WriteString(`<s:Body>`)
	sb.WriteString(fmt.Sprintf(`<u:%sResponse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1">`, action))
	for k, v := range params {
		sb.WriteString(fmt.Sprintf(`<%s>%s</%s>`, k, xmlEscape(v), k))
	}
	sb.WriteString(fmt.Sprintf(`</u:%sResponse>`, action))
	sb.WriteString(`</s:Body></s:Envelope>`)
	w.Write([]byte(sb.String()))
}

func (cd *ContentDirectoryService) soapError(w http.ResponseWriter, code int, desc string) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
<s:Body><s:Fault><faultcode>s:Client</faultcode><faultstring>UPnPError</faultstring>
<detail><UPnPError xmlns="urn:schemas-upnp-org:control-1-0">
<errorCode>%d</errorCode><errorDescription>%s</errorDescription>
</UPnPError></detail></s:Fault></s:Body></s:Envelope>`, code, desc)))
}

func xmlEscape(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

func parseSOAPBody(r *http.Request) (string, error) {
	buf := make([]byte, 8192)
	n, err := r.Body.Read(buf)
	if err != nil && n == 0 {
		return "", err
	}
	return string(buf[:n]), nil
}

func extractXMLTag(body, tag string) string {
	start := strings.Index(body, "<"+tag+">")
	end := strings.Index(body, "</"+tag+">")
	if start < 0 || end < 0 {
		return ""
	}
	return body[start+len(tag)+2 : end]
}

// Suppress unused import
var _ = uuid.New

const contentDirectorySCPD = `<?xml version="1.0" encoding="UTF-8"?>
<scpd xmlns="urn:schemas-upnp-org:service-1-0">
<specVersion><major>1</major><minor>0</minor></specVersion>
<actionList>
<action><name>Browse</name>
<argumentList>
<argument><name>ObjectID</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_ObjectID</relatedStateVariable></argument>
<argument><name>BrowseFlag</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_BrowseFlag</relatedStateVariable></argument>
<argument><name>Filter</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_Filter</relatedStateVariable></argument>
<argument><name>StartingIndex</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_Index</relatedStateVariable></argument>
<argument><name>RequestedCount</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_Count</relatedStateVariable></argument>
<argument><name>SortCriteria</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_SortCriteria</relatedStateVariable></argument>
<argument><name>Result</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_Result</relatedStateVariable></argument>
<argument><name>NumberReturned</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_Count</relatedStateVariable></argument>
<argument><name>TotalMatches</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_Count</relatedStateVariable></argument>
<argument><name>UpdateID</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_UpdateID</relatedStateVariable></argument>
</argumentList>
</action>
<action><name>GetSystemUpdateID</name>
<argumentList>
<argument><name>Id</name><direction>out</direction><relatedStateVariable>SystemUpdateID</relatedStateVariable></argument>
</argumentList>
</action>
<action><name>Search</name>
<argumentList>
<argument><name>ContainerID</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_ObjectID</relatedStateVariable></argument>
<argument><name>SearchCriteria</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_SearchCriteria</relatedStateVariable></argument>
<argument><name>Filter</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_Filter</relatedStateVariable></argument>
<argument><name>StartingIndex</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_Index</relatedStateVariable></argument>
<argument><name>RequestedCount</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_Count</relatedStateVariable></argument>
<argument><name>SortCriteria</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_SortCriteria</relatedStateVariable></argument>
<argument><name>Result</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_Result</relatedStateVariable></argument>
<argument><name>NumberReturned</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_Count</relatedStateVariable></argument>
<argument><name>TotalMatches</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_Count</relatedStateVariable></argument>
<argument><name>UpdateID</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_UpdateID</relatedStateVariable></argument>
</argumentList>
</action>
</actionList>
<serviceStateTable>
<stateVariable sendEvents="yes"><name>SystemUpdateID</name><dataType>ui4</dataType></stateVariable>
<stateVariable sendEvents="no"><name>A_ARG_TYPE_ObjectID</name><dataType>string</dataType></stateVariable>
<stateVariable sendEvents="no"><name>A_ARG_TYPE_Result</name><dataType>string</dataType></stateVariable>
<stateVariable sendEvents="no"><name>A_ARG_TYPE_BrowseFlag</name><dataType>string</dataType><allowedValueList><allowedValue>BrowseMetadata</allowedValue><allowedValue>BrowseDirectChildren</allowedValue></allowedValueList></stateVariable>
<stateVariable sendEvents="no"><name>A_ARG_TYPE_Filter</name><dataType>string</dataType></stateVariable>
<stateVariable sendEvents="no"><name>A_ARG_TYPE_SortCriteria</name><dataType>string</dataType></stateVariable>
<stateVariable sendEvents="no"><name>A_ARG_TYPE_SearchCriteria</name><dataType>string</dataType></stateVariable>
<stateVariable sendEvents="no"><name>A_ARG_TYPE_Index</name><dataType>ui4</dataType></stateVariable>
<stateVariable sendEvents="no"><name>A_ARG_TYPE_Count</name><dataType>ui4</dataType></stateVariable>
<stateVariable sendEvents="no"><name>A_ARG_TYPE_UpdateID</name><dataType>ui4</dataType></stateVariable>
</serviceStateTable>
</scpd>`
