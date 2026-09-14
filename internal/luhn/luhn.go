package luhn

func Valid(number string) bool {
	if number == "" {
		return false
	}
	sum := 0
	for i := len(number) - 1; i >= 0; i-- {
		ch := number[i]
		if ch < '0' || ch > '9' {
			return false
		}

		digit := int(ch - '0')
		position := len(number) - 1 - i

		if position%2 == 0 {
			sum += digit
		} else {
			digit = digit * 2
			if digit > 9 {
				digit -= 9
			}
			sum += digit
		}
	}
	return sum%10 == 0
}
