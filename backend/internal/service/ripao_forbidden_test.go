package service

import "testing"

func TestIsDailyThrowawayGroup(t *testing.T) {
	if IsDailyThrowawayGroup(&Group{Name: "pro号池(严禁破限)"}) {
		t.Fatal("pro group is not 日抛")
	}
	if !IsDailyThrowawayGroup(&Group{Name: "日抛"}) {
		t.Fatal("日抛 group must match")
	}
	if IsDailyThrowawayGroup(nil) {
		t.Fatal("nil group")
	}
}

func TestIsDailyThrowawayForbiddenAccount(t *testing.T) {
	if !IsDailyThrowawayForbiddenAccount(&Account{Name: "zbj9786@gmail.com"}) {
		t.Fatal("zbj")
	}
	if !IsDailyThrowawayForbiddenAccount(&Account{Name: "team-beilegedong@gmail.com-ws-1"}) {
		t.Fatal("beilegedong suffix")
	}
	if !IsDailyThrowawayForbiddenAccount(&Account{Name: "cuadrasdeshon335@gmail.com"}) {
		t.Fatal("cuadras")
	}
	if IsDailyThrowawayForbiddenAccount(&Account{Name: "marynelsonc924+zh2fp2c@gmail.com"}) {
		t.Fatal("mary is a 日抛 car")
	}
}

func TestFilterDailyThrowawayForbiddenAccounts(t *testing.T) {
	ripao := &Group{Name: "日抛"}
	pro := &Group{Name: "pro号池(严禁破限)"}
	zbj := &Account{ID: 1, Name: "zbj9786@gmail.com"}
	mary := &Account{ID: 2, Name: "marynelsonc924+zh2fp2c@gmail.com"}
	got := filterDailyThrowawayForbiddenAccounts(ripao, []*Account{zbj, mary})
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("日抛 must drop zbj, got %+v", got)
	}
	got = filterDailyThrowawayForbiddenAccounts(pro, []*Account{zbj, mary})
	if len(got) != 2 {
		t.Fatalf("pro may still use zbj, got %d", len(got))
	}
}
