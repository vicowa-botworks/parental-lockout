package web

import (
	"net/http/httptest"
	"testing"
)

// The property this name describes is a property of one function, and the end-to-end
// version of it (TestAForwardedForHeaderDoesNotChooseTheThrottleBucket) only notices a
// regression by watching a bucket fill -- which means it can pass while the header IS
// consulted, as long as the bucket still fills for some other reason. Asserted here
// directly, it fails the moment anyone adds an X-Forwarded-For branch.
//
// No timing, no penalty, no clock.
func TestTheThrottleKeyIsThePeerAddressAndNothingElse(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{})

	cases := []struct {
		name       string
		remoteAddr string
		forwarded  []string
		want       string
	}{
		{
			name:       "a header naming some other victim",
			remoteAddr: "192.0.2.30:52134",
			forwarded:  []string{"203.0.113.7"},
			want:       "192.0.2.30",
		},
		{
			name:       "a header chain, the way a real proxy would send one",
			remoteAddr: "192.0.2.30:52134",
			forwarded:  []string{"203.0.113.7, 198.51.100.9", "203.0.113.8"},
			want:       "192.0.2.30",
		},
		{
			name:       "an empty header, which is what a client that is not attacking sends",
			remoteAddr: "192.0.2.30:52134",
			forwarded:  []string{""},
			want:       "192.0.2.30",
		},
		{
			name:       "a spoofed loopback origin",
			remoteAddr: "192.0.2.30:52134",
			forwarded:  []string{"127.0.0.1"},
			want:       "192.0.2.30",
		},
		{
			name:       "an IPv6 zone, which is the same host twice",
			remoteAddr: "[fe80::1%enp3s0]:52134",
			want:       "fe80::1",
		},
		{
			name:       "no peer address at all",
			remoteAddr: "",
			want:       "unknown",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/login", nil)
			req.RemoteAddr = tc.remoteAddr
			for _, v := range tc.forwarded {
				req.Header.Add("X-Forwarded-For", v)
			}
			if got := s.throttleKey(req); got != tc.want {
				t.Errorf("throttleKey = %q, want %q", got, tc.want)
			}
		})
	}
}

// The control that keeps the table above from passing because throttleKey ignores
// everything: the peer address itself has to reach the bucket, or the throttle would
// refuse the whole LAN on one failure.
func TestTheThrottleKeyFollowsThePeerAddress(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{})

	seen := map[string]bool{}
	for _, addr := range []string{"192.0.2.30:1", "192.0.2.31:1", "198.51.100.4:9"} {
		req := httptest.NewRequest("POST", "/login", nil)
		req.RemoteAddr = addr
		seen[s.throttleKey(req)] = true
	}
	if len(seen) != 3 {
		t.Errorf("three peers produced %d buckets (%v): one failure would throttle the subnet", len(seen), seen)
	}
}
