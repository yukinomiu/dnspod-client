package modifier

import (
	"sort"
	"strings"
)

type (
	KeyValuePair struct {
		Key   string
		Value string
	}

	CanonicalHeaders []*KeyValuePair
)

func NewCanonicalHeaders(headers []*KeyValuePair) CanonicalHeaders {
	ch := CanonicalHeaders(headers)

	// trim and to lower case
	for _, header := range ch {
		header.Key = strings.TrimSpace(strings.ToLower(header.Key))
		header.Value = strings.TrimSpace(strings.ToLower(header.Value))
	}

	// sort
	sort.Sort(ch)

	return ch
}

func (ch CanonicalHeaders) Len() int {
	return len(ch)
}

func (ch CanonicalHeaders) Less(i, j int) bool {
	return strings.Compare(ch[i].Key, ch[j].Key) < 0
}

func (ch CanonicalHeaders) Swap(i, j int) {
	ch[i], ch[j] = ch[j], ch[i]
}

func (ch CanonicalHeaders) ToCanonicalHeaders() string {
	if len(ch) == 0 {
		return ""
	}

	// join
	builder := &strings.Builder{}
	for _, header := range ch {
		builder.WriteString(header.Key)
		builder.WriteString(":")
		builder.WriteString(header.Value)
		builder.WriteString("\n")
	}

	return builder.String()
}

func (ch CanonicalHeaders) ToSignedHeaders() string {
	if len(ch) == 0 {
		return ""
	}

	builder := &strings.Builder{}
	for i, header := range ch {
		builder.WriteString(header.Key)
		if i != len(ch)-1 {
			builder.WriteString(";")
		}
	}

	return builder.String()
}
