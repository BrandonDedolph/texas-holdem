package tutorial

import (
	"strings"
	"testing"
)

func TestChoiceAnswerCheck(t *testing.T) {
	a := ChoiceAnswer{Choices: []string{"Fold", "Call", "Raise"}, Correct: 1}
	for _, in := range []string{"2", " 2 ", "call", "CALL", "Call"} {
		if !a.Check(in) {
			t.Errorf("Check(%q) = false, want true", in)
		}
	}
	for _, in := range []string{"1", "3", "fold", "", "call it", "0"} {
		if a.Check(in) {
			t.Errorf("Check(%q) = true, want false", in)
		}
	}
}

func TestNumericAnswerCheck(t *testing.T) {
	exact := NumericAnswer{Value: 9}
	for _, in := range []string{"9", " 9 ", "9.0", "9%"} {
		if !exact.Check(in) {
			t.Errorf("exact Check(%q) = false, want true", in)
		}
	}
	for _, in := range []string{"8", "10", "nine", ""} {
		if exact.Check(in) {
			t.Errorf("exact Check(%q) = true, want false", in)
		}
	}

	loose := NumericAnswer{Value: 36, Tolerance: 5}
	for _, in := range []string{"31", "36", "41", "35.5", "40%"} {
		if !loose.Check(in) {
			t.Errorf("±5 Check(%q) = false, want true", in)
		}
	}
	for _, in := range []string{"30", "42", "50"} {
		if loose.Check(in) {
			t.Errorf("±5 Check(%q) = true, want false", in)
		}
	}
}

func TestOrderAnswerCheck(t *testing.T) {
	a := OrderAnswer{
		Items:   []string{"flush", "full house", "straight"},
		Correct: []int{1, 0, 2},
	}
	for _, in := range []string{"2 1 3", "2,1,3", " 2, 1, 3 "} {
		if !a.Check(in) {
			t.Errorf("Check(%q) = false, want true", in)
		}
	}
	for _, in := range []string{"1 2 3", "2 1", "2 1 3 1", "", "a b c"} {
		if a.Check(in) {
			t.Errorf("Check(%q) = true, want false", in)
		}
	}
}

// TestFactsRejectWrongAnswers plants deliberately wrong answers and asserts
// each Fact type catches them — the guarantee that a lesson can't teach a
// wrong poker fact and still pass the content tests.
func TestFactsRejectWrongAnswers(t *testing.T) {
	cases := []struct {
		name  string
		drill Drill
		want  string // substring of the expected error; "" means must pass
	}{
		{
			name: "winner fact catches wrong winner",
			drill: Drill{
				Answer: ChoiceAnswer{Choices: []string{"A", "B", "Split"}, Correct: 0},
				Fact: WinnerFact{
					Board: "Qs Jh 7d 4c 2h",
					Hands: []string{"Ac Kd", "7h 8h", ""}, // B wins (pair of sevens)
					Split: 2,
				},
			},
			want: "winner",
		},
		{
			name: "winner fact accepts right winner",
			drill: Drill{
				Answer: ChoiceAnswer{Choices: []string{"A", "B", "Split"}, Correct: 1},
				Fact: WinnerFact{
					Board: "Qs Jh 7d 4c 2h",
					Hands: []string{"Ac Kd", "7h 8h", ""},
					Split: 2,
				},
			},
		},
		{
			name: "winner fact catches missed split",
			drill: Drill{
				Answer: ChoiceAnswer{Choices: []string{"A", "B", "Split"}, Correct: 0},
				Fact: WinnerFact{
					Board: "7c 8d 9h Tc Js", // board straight plays for both
					Hands: []string{"Ad 2c", "Kh 3d", ""},
					Split: 2,
				},
			},
			want: "winner",
		},
		{
			name: "outs fact catches wrong count",
			drill: Drill{
				Answer: NumericAnswer{Value: 8}, // flush draw is 9
				Fact:   OutsFact{Hero: "9s 8s", Board: "As Ks 2h"},
			},
			want: "outs",
		},
		{
			name: "outs fact rejects tolerance",
			drill: Drill{
				Answer: NumericAnswer{Value: 9, Tolerance: 1},
				Fact:   OutsFact{Hero: "9s 8s", Board: "As Ks 2h"},
			},
			want: "exact",
		},
		{
			name: "pot odds fact catches wrong percent",
			drill: Drill{
				Answer: NumericAnswer{Value: 20, Tolerance: 1}, // 50 into 150 is 25%
				Fact:   PotOddsFact{Pot: 150, Call: 50},
			},
			want: "recomputed",
		},
		{
			name: "equity fact catches a wild value",
			drill: Drill{
				Answer: NumericAnswer{Value: 60, Tolerance: 5}, // flush draw is ~37%
				Fact: EquityFact{
					Hero: "9s 8s", Villain: "Ad Qd", Board: "As Ks 2h",
				},
			},
			want: "equity",
		},
		{
			name: "range fact catches wrong membership",
			drill: Drill{
				Answer: ChoiceAnswer{Choices: []string{"Open", "Fold"}, Correct: 1},
				Fact: RangeFact{
					Spec: "A2s+", Hand: "As 5s", // A5s IS in A2s+
					InChoice: 0, OutChoice: 1,
				},
			},
			want: "membership",
		},
		{
			name: "rank order fact catches wrong order",
			drill: Drill{
				Answer: OrderAnswer{
					Items:   []string{"Ad Td 8d 6d 3d", "Kc Kh Kd 4s 4d"},
					Correct: []int{0, 1}, // flush before full house: wrong
				},
				Fact: RankOrderFact{Hands: []string{"Ad Td 8d 6d 3d", "Kc Kh Kd 4s 4d"}},
			},
			want: "ahead",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.drill.Fact.Verify(&tc.drill)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("Verify() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Verify() = nil, want error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Verify() = %v, want mention of %q", err, tc.want)
			}
		})
	}
}
