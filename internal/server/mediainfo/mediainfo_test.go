package mediainfo

import (
	"context"
	"io"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func TestGetBitrateTestBytes(t *testing.T) {
	for _, tc := range []struct {
		name string
		size *int32
		want int64
	}{
		{name: "default", want: defaultBitrateTestSize},
		{name: "requested size", size: apiutil.Ptr(int32(4096)), want: 4096},
		{name: "capped", size: apiutil.Ptr(int32(maxBitrateTestSize + 1)), want: maxBitrateTestSize},
		{name: "zero", size: apiutil.Ptr(int32(0)), want: 1},
		{name: "negative", size: apiutil.Ptr(int32(-5)), want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := New(nil).GetBitrateTestBytes(context.Background(), api.GetBitrateTestBytesRequestObject{
				Params: api.GetBitrateTestBytesParams{Size: tc.size},
			})
			if err != nil {
				t.Fatal(err)
			}

			body := response.(api.GetBitrateTestBytes200ApplicationoctetStreamResponse)
			if body.ContentLength != tc.want {
				t.Errorf("got content length %d, want %d", body.ContentLength, tc.want)
			}

			written, err := io.Copy(io.Discard, body.Body)
			if err != nil {
				t.Fatal(err)
			}
			if written != tc.want {
				t.Errorf("got %d bytes, want %d", written, tc.want)
			}
		})
	}
}
