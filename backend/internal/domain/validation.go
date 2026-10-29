package domain

import (
	"math"
	"regexp"
	"strings"
	"unicode"
)

const MaxMoney int64 = 9007199254740991

var cpfPattern = regexp.MustCompile(`^[0-9]{11}$`)
var cnpjPattern = regexp.MustCompile(`^[A-Z0-9]{12}[0-9]{2}$`)
var platePattern = regexp.MustCompile(`^[A-Z]{3}[0-9][A-Z0-9][0-9]{2}$`)

func Normalize(s string) string {
	return strings.ToUpper(strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune(".-/", r) {
			return -1
		}
		return r
	}, s))
}
func Document(kind, input string) (string, error) {
	s := Normalize(input)
	if kind == "CPF" {
		if !cpfPattern.MatchString(s) || s == strings.Repeat(s[:1], 11) {
			return "", Invalid("CPF inválido.")
		}
		for n := 9; n <= 10; n++ {
			sum := 0
			for i := 0; i < n; i++ {
				sum += int(s[i]-'0') * (n + 1 - i)
			}
			dv := (sum * 10) % 11
			if dv == 10 {
				dv = 0
			}
			if int(s[n]-'0') != dv {
				return "", Invalid("CPF inválido.")
			}
		}
	} else if kind == "CNPJ" {
		if !cnpjPattern.MatchString(s) || s == strings.Repeat(s[:1], 14) {
			return "", Invalid("CNPJ inválido.")
		}
		for n := 12; n <= 13; n++ {
			sum, w := 0, 2
			for i := n - 1; i >= 0; i-- {
				sum += int(s[i]-'0') * w
				w++
				if w == 10 {
					w = 2
				}
			}
			dv := 11 - sum%11
			if dv >= 10 {
				dv = 0
			}
			if int(s[n]-'0') != dv {
				return "", Invalid("CNPJ inválido.")
			}
		}
	} else {
		return "", Invalid("Tipo de documento deve ser CPF ou CNPJ.")
	}
	return s, nil
}
func Plate(input string) (string, error) {
	s := Normalize(input)
	if !platePattern.MatchString(s) {
		return "", Invalid("Placa brasileira inválida.")
	}
	return s, nil
}
func AddMoney(a, b int64) (int64, error) {
	if a < 0 || b < 0 || a > MaxMoney || b > MaxMoney-a {
		return 0, Invalid("Total excede o limite monetário.")
	}
	return a + b, nil
}
func MultiplyMoney(price, quantity int64) (int64, error) {
	if price < 0 || price > MaxMoney || quantity < 1 || (price > 0 && quantity > MaxMoney/price) {
		return 0, Invalid("Valor ou quantidade excede o limite monetário.")
	}
	return price * quantity, nil
}
func AdjustStock(balance, delta int64) (int64, error) {
	if delta == 0 {
		return 0, Invalid("Movimento deve ter quantidade diferente de zero.")
	}
	if delta > 0 && balance > math.MaxInt64-delta {
		return 0, Invalid("Saldo excede o limite permitido.")
	}
	if delta < 0 && (delta == math.MinInt64 || balance < -delta) {
		return 0, Conflict("INSUFFICIENT_STOCK", "Estoque insuficiente.")
	}
	return balance + delta, nil
}
