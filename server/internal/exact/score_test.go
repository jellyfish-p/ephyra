package exact

import "testing"

func TestParse(t *testing.T) {
	ok := []struct {
		in   string
		want string
	}{
		{"0", "0.00"},
		{"50", "50.00"},
		{"50.5", "50.50"},
		{"50.50", "50.50"},
		{"1.29", "1.29"},
		{"-2.5", "-2.50"},
	}
	for _, tc := range ok {
		got, err := Parse(tc.in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.in, err)
		}
		if got.String() != tc.want {
			t.Fatalf("Parse(%q)=%s want %s", tc.in, got.String(), tc.want)
		}
	}
	bad := []string{"", "1e2", "1.234", "50.000", "01", "1.", ".5", "++1", "abc"}
	for _, in := range bad {
		if _, err := Parse(in); err == nil {
			t.Fatalf("Parse(%q) accepted", in)
		}
	}
}

func TestAddCmp(t *testing.T) {
	a := FromInt(50)
	b := FromInt(50)
	if a.Add(b).String() != "100.00" {
		t.Fatal(a.Add(b))
	}
	if a.Cmp(FromInt(60)) != -1 || a.Cmp(b) != 0 {
		t.Fatal("cmp")
	}
}
