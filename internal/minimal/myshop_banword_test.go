package minimal

import "testing"

func TestMyShopOpenSignContainsBanwordSubstringMatch(t *testing.T) {
	cleanup := SetMyShopOpenBanwordsForTest([]string{"banme", "금지어"})
	defer cleanup()

	for _, tc := range []struct {
		name string
		sign string
		want bool
	}{
		{name: "exact ascii", sign: "banme", want: true},
		{name: "embedded ascii", sign: "My banme Shop", want: true},
		{name: "case sensitive miss", sign: "BanMe", want: false},
		{name: "clean ascii", sign: "Private Shop", want: false},
		{name: "exact multibyte", sign: "금지어", want: true},
		{name: "embedded multibyte", sign: "상점 금지어 이름", want: true},
		{name: "empty sign", sign: "", want: false},
	} {
		if got := myShopOpenSignContainsBanword(tc.sign); got != tc.want {
			t.Fatalf("%s: myShopOpenSignContainsBanword(%q)=%v want %v", tc.name, tc.sign, got, tc.want)
		}
	}
}

func TestMyShopOpenSignContainsBanwordEmptyListNeverMatches(t *testing.T) {
	cleanup := SetMyShopOpenBanwordsForTest([]string{})
	defer cleanup()
	if myShopOpenSignContainsBanword("banme") {
		t.Fatal("empty banword list must never match")
	}
}

func TestMyShopOpenSignContainsBanwordBootstrapDefaults(t *testing.T) {
	cleanup := SetMyShopOpenBanwordsForTest(nil)
	defer cleanup()
	if !myShopOpenSignContainsBanword("please banme now") {
		t.Fatal("expected bootstrap ascii banword to match")
	}
	if !myShopOpenSignContainsBanword("금지어 shop") {
		t.Fatal("expected bootstrap multibyte banword to match")
	}
	if myShopOpenSignContainsBanword("Private Shop") {
		t.Fatal("clean sign must not match bootstrap banwords")
	}
}
