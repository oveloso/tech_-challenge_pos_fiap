package domain

import (
	"math"
	"testing"
)

func TestDocument(t *testing.T) {
	for _, tc := range []struct {
		kind, input, want string
		valid             bool
	}{
		{"CPF", "529.982.247-25", "52998224725", true}, {"CPF", "11111111111", "", false}, {"CPF", "52998224724", "", false}, {"CPF", "52998224735", "", false}, {"CPF", "", "", false}, {"CPF", "ABC98224725", "", false},
		{"CNPJ", "11.222.333/0001-81", "11222333000181", true}, {"CNPJ", "12.abc.345/01de-35", "12ABC34501DE35", true}, {"CNPJ", "00000000000000", "", false}, {"CNPJ", "12ABC34501DE34", "", false}, {"CNPJ", "12ABC34501DE45", "", false}, {"CNPJ", "12ABC34501DEAA", "", false}, {"CNPJ", "ABC", "", false}, {"RG", "52998224725", "", false},
	} {
		t.Run(tc.kind+tc.input, func(t *testing.T) {
			got, e := Document(tc.kind, tc.input)
			if (e == nil) != tc.valid || got != tc.want {
				t.Fatalf("got %q, %v", got, e)
			}
		})
	}
}
func TestPlate(t *testing.T) {
	for _, tc := range []struct {
		input, want string
		valid       bool
	}{{"abc-1234", "ABC1234", true}, {"abc1d23", "ABC1D23", true}, {"ABC123", "", false}, {"AB12345", "", false}, {"ABCAD12", "", false}, {"ABC1D2Z", "", false}} {
		got, e := Plate(tc.input)
		if (e == nil) != tc.valid || got != tc.want {
			t.Errorf("%q: %q %v", tc.input, got, e)
		}
	}
}
func TestMoneyBoundaries(t *testing.T) {
	for _, tc := range []struct {
		a, b, want int64
		valid      bool
	}{{0, 0, 0, true}, {MaxMoney - 1, 1, MaxMoney, true}, {MaxMoney, 1, 0, false}, {-1, 1, 0, false}, {1, -1, 0, false}, {MaxMoney + 1, 0, 0, false}} {
		got, e := AddMoney(tc.a, tc.b)
		if got != tc.want || (e == nil) != tc.valid {
			t.Errorf("sum %+v = %d, %v", tc, got, e)
		}
	}
	for _, tc := range []struct {
		price, q, want int64
		valid          bool
	}{{100, 3, 300, true}, {0, math.MaxInt64, 0, true}, {MaxMoney, 1, MaxMoney, true}, {MaxMoney, 2, 0, false}, {1, 0, 0, false}, {-1, 1, 0, false}, {MaxMoney + 1, 1, 0, false}} {
		got, e := MultiplyMoney(tc.price, tc.q)
		if got != tc.want || (e == nil) != tc.valid {
			t.Errorf("multiply %+v = %d, %v", tc, got, e)
		}
	}
}
func TestStockBoundaries(t *testing.T) {
	for _, tc := range []struct {
		balance, delta, want int64
		valid                bool
	}{{5, -5, 0, true}, {0, 8, 8, true}, {5, -6, 0, false}, {1, 0, 0, false}, {1, math.MinInt64, 0, false}, {math.MaxInt64, 1, 0, false}} {
		got, e := AdjustStock(tc.balance, tc.delta)
		if got != tc.want || (e == nil) != tc.valid {
			t.Errorf("stock %+v = %d, %v", tc, got, e)
		}
	}
}
func TestDomainErrors(t *testing.T) {
	for _, e := range []error{Invalid("x"), Conflict("C", "x"), Missing(), Unauthorized()} {
		if e.Error() == "" {
			t.Fatal("empty detail")
		}
	}
}
