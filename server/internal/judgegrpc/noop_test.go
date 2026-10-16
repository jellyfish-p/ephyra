package judgegrpc

import "testing"

func TestServerTypeExists(t *testing.T) {
	var s Server
	if s.Judging != nil {
		t.Fatal("zero server has no judging module")
	}
}
