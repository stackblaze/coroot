package tracing

import (
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/stretchr/testify/assert"
)

func TestFilterSpans(t *testing.T) {
	spans := []*model.TraceSpan{{ServiceName: "a"}, {ServiceName: "b"}, {ServiceName: "a"}}
	out := filterSpans(spans, func(s string) bool { return s == "a" })
	assert.Len(t, out, 2)
	for _, s := range out {
		assert.Equal(t, "a", s.ServiceName)
	}
}
