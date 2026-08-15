package provider

import (
	"bytes"
	"encoding/json"
	"io"
	"unicode/utf8"

	"github.com/uinaf/autoreview/internal/protocol"
)

func reviewDocumentReason(data []byte) protocol.ProtocolReason {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || !utf8.Valid(data) {
		return protocol.ProtocolReasonInvalidJSON
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var first json.RawMessage
	if err := decoder.Decode(&first); err != nil {
		return protocol.ProtocolReasonInvalidJSON
	}
	first = bytes.TrimSpace(first)
	if len(first) == 0 || first[0] != '{' {
		return protocol.ProtocolReasonInvalidDocumentShape
	}
	var second json.RawMessage
	switch err := decoder.Decode(&second); {
	case err == nil:
		return protocol.ProtocolReasonMultipleDocuments
	case err != io.EOF:
		return protocol.ProtocolReasonSuffixContent
	default:
		return protocol.ProtocolReasonSchemaMismatch
	}
}

func (result Result) ResolvedExecution() Execution {
	return Execution{
		Provider:         result.Provider,
		Isolation:        result.Isolation,
		WebAccess:        result.WebAccess,
		ProtocolRecovery: result.ProtocolRecovery,
	}
}
